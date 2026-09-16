package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/api/option"
)

var client *messaging.Client
var collection *mongo.Collection

// TokenRequest now includes an optional Channel field.
// If no channel is given, the token is registered under "default".
type TokenRequest struct {
	Token   string `json:"token"`
	Channel string `json:"channel"`
}

// SendRequest lets the caller customize the notification content
// and choose which channel to broadcast to.
type SendRequest struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	Channel string `json:"channel"`
}

func enableCORS(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

func initFirebase() {
	cred := os.Getenv("FIREBASE_CREDENTIALS")
	opt := option.WithCredentialsJSON([]byte(cred))

	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Fatal(err)
	}

	client, err = app.Messaging(context.Background())
	if err != nil {
		log.Fatal(err)
	}
}

func initDB() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoURI := os.Getenv("MONGODB_URI")

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal(err)
	}

	collection = mongoClient.Database("fcm_db").Collection("tokens")
}

// saveToken now upserts by token, and stores which channel/room
// the device belongs to. Sending the same token twice just updates
// its channel instead of creating a duplicate row.
func saveToken(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)

	if r.Method == http.MethodOptions {
		return
	}

	var req TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	channel := req.Channel
	if channel == "" {
		channel = "default"
	}

	filter := bson.M{"token": req.Token}
	update := bson.M{
		"$set": bson.M{
			"token":   req.Token,
			"channel": channel,
		},
	}
	opts := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		http.Error(w, "failed to save token", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Token saved to channel: %s\n", channel)
}

// sendNotification now:
//  1. accepts a custom title/body instead of hardcoding one
//  2. only sends to tokens in the requested channel (or "default")
//  3. removes tokens from the DB if FCM reports them as
//     invalid/unregistered, so dead devices don't pile up
func sendNotification(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)

	if r.Method == http.MethodOptions {
		return
	}

	var req SendRequest
	json.NewDecoder(r.Body).Decode(&req) // ignore error: fall back to defaults below

	title := req.Title
	if title == "" {
		title = "Hello"
	}
	body := req.Body
	if body == "" {
		body = "Sent from Go backend"
	}
	channel := req.Channel
	if channel == "" {
		channel = "default"
	}

	cursor, err := collection.Find(context.Background(), bson.M{"channel": channel})
	if err != nil {
		http.Error(w, "failed to query tokens", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	sent := 0
	failed := 0

	for cursor.Next(context.Background()) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		token, ok := doc["token"].(string)
		if !ok {
			continue
		}

		message := &messaging.Message{
			Notification: &messaging.Notification{
				Title: title,
				Body:  body,
			},
			Token: token,
		}

		_, err := client.Send(context.Background(), message)
		if err != nil {
			failed++
			log.Printf("failed to send to token %s: %v", token, err)

			// Clean up tokens FCM says are dead so they stop
			// being retried on every future broadcast.
			if messaging.IsRegistrationTokenNotRegistered(err) ||
				messaging.IsInvalidArgument(err) {
				collection.DeleteOne(context.Background(), bson.M{"token": token})
				log.Printf("removed dead token: %s", token)
			}
			continue
		}

		sent++
	}

	fmt.Fprintf(w, "Sent to channel '%s': %d succeeded, %d failed\n", channel, sent, failed)
}

func main() {
	initFirebase()
	initDB()

	http.HandleFunc("/save-token", saveToken)
	http.HandleFunc("/send", sendNotification)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Running on", port)
	http.ListenAndServe("0.0.0.0:"+port, nil)
}