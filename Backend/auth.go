package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

// A separate collection just for users, alongside your existing
// `collection` (tokens). Set once in initAuth(), called from main().
var usersCollection *mongo.Collection

// User is what gets stored in MongoDB for each account.
// We never store the plain password, only its bcrypt hash.
type User struct {
	Email        string `bson:"email" json:"email"`
	PasswordHash string `bson:"passwordHash" json:"-"`
}

type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// initAuth points usersCollection at a "users" table in the same
// database your tokens collection already lives in.
func initAuth() {
	usersCollection = collection.Database().Collection("users")
}

// signup handles POST /signup — creates a new account.
func signup(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == http.MethodOptions {
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}

	// Reject if the email is already taken.
	existing := usersCollection.FindOne(context.Background(), bson.M{"email": req.Email})
	if existing.Err() == nil {
		http.Error(w, "an account with this email already exists", http.StatusConflict)
		return
	}

	// bcrypt.GenerateFromPassword turns the plain password into a
	// one-way hash. Even if your database is ever leaked, the
	// original password can't be recovered from this hash.
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to process password", http.StatusInternalServerError)
		return
	}

	_, err = usersCollection.InsertOne(context.Background(), User{
		Email:        req.Email,
		PasswordHash: string(hash),
	})
	if err != nil {
		http.Error(w, "failed to create account", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Account created. You can now log in.\n"))
}

// login handles POST /login — verifies credentials and returns a JWT.
func login(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == http.MethodOptions {
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}

	var user User
	err := usersCollection.FindOne(context.Background(), bson.M{"email": req.Email}).Decode(&user)
	if err != nil {
		// Deliberately vague — don't reveal whether the email exists.
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	// bcrypt.CompareHashAndPassword checks the plain password against
	// the stored hash. It returns an error if they don't match.
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	token, err := generateJWT(user.Email)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// generateJWT creates a signed token that proves who the user is,
// without your server needing to remember active sessions itself.
func generateJWT(email string) (string, error) {
	secret := os.Getenv("JWT_SECRET")

	claims := jwt.MapClaims{
		"email": email,
		"exp":   time.Now().Add(24 * time.Hour).Unix(), // expires in 24h
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// requireAuth wraps a handler so it only runs if the request has a
// valid "Authorization: Bearer <token>" header. If the token is
// missing, malformed, expired, or has a bad signature, the request
// is rejected before your actual handler ever runs.
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == http.MethodOptions {
			return
		}

		authHeader := r.Header.Get("Authorization")
		if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
			http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}
		tokenString := authHeader[7:]

		secret := os.Getenv("JWT_SECRET")
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Token is valid — proceed to the real handler.
		next(w, r)
	}
}