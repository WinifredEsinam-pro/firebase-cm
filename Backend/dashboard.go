package main

import (
	"context"
	"encoding/json"
	"net/http"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ChannelSummary is what /channels returns for each distinct channel.
type ChannelSummary struct {
	Channel string `json:"channel" bson:"_id"`
	Devices int    `json:"devices" bson:"devices"`
}

// getChannels handles GET /channels — returns every distinct channel
// currently in use, along with how many devices are registered under it.
func getChannels(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == http.MethodOptions {
		return
	}

	// This is a MongoDB aggregation pipeline — think of it as a
	// small sequence of steps run inside the database itself:
	//   $group: bucket every token document by its "channel" field,
	//           and count how many documents land in each bucket.
	//   $sort:  order the results so the busiest channels show first.
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":     "$channel",
			"devices": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"devices": -1}}},
	}

	cursor, err := collection.Aggregate(context.Background(), pipeline)
	if err != nil {
		http.Error(w, "failed to load channels", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	var results []ChannelSummary
	if err := cursor.All(context.Background(), &results); err != nil {
		http.Error(w, "failed to read channels", http.StatusInternalServerError)
		return
	}

	// Always return an array, even if empty, so the frontend
	// doesn't have to special-case "null" vs "no channels yet".
	if results == nil {
		results = []ChannelSummary{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// getHistory handles GET /history — returns recent sent broadcasts,
// optionally filtered to one channel via ?channel=room1
func getHistory(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == http.MethodOptions {
		return
	}

	filter := bson.M{}
	if ch := r.URL.Query().Get("channel"); ch != "" {
		filter["channel"] = ch
	}

	// Most recent first, capped at 50 so this never returns an
	// unbounded amount of data as history grows.
	opts := options.Find().SetSort(bson.M{"timestamp": -1}).SetLimit(50)

	cursor, err := messagesCollection.Find(context.Background(), filter, opts)
	if err != nil {
		http.Error(w, "failed to load history", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	var results []bson.M
	if err := cursor.All(context.Background(), &results); err != nil {
		http.Error(w, "failed to read history", http.StatusInternalServerError)
		return
	}

	if results == nil {
		results = []bson.M{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}