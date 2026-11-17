// migration_google_keys reads valid Google credentials for the migration.
// Its JSON output is intended only for a local pipe and must not be logged.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "MongoDB key lookup failed:", err)
		os.Exit(1)
	}
}

func run() error {
	uri, dbName := os.Getenv("MONGO_URI"), os.Getenv("MONGO_DB_NAME")
	if uri == "" || dbName == "" {
		return fmt.Errorf("MONGO_URI and MONGO_DB_NAME are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return fmt.Errorf("connect to MongoDB: %w", err)
	}
	defer client.Disconnect(context.Background())
	collection := client.Database(dbName).Collection("api_keys")
	cursor, err := collection.Find(ctx,
		bson.M{"provider": "Google", "status": "Valid"},
		options.Find().SetProjection(bson.M{"key_value": 1}).SetSort(bson.D{{Key: "created_at", Value: -1}}),
	)
	if err != nil {
		return fmt.Errorf("query valid Google keys: %w", err)
	}
	defer cursor.Close(ctx)
	keys := make([]string, 0)
	seen := make(map[string]bool)
	for cursor.Next(ctx) {
		var row struct {
			Key string `bson:"key_value"`
		}
		if err := cursor.Decode(&row); err != nil {
			return fmt.Errorf("decode Google key: %w", err)
		}
		key := strings.TrimSpace(row.Key)
		if key != "" && !seen[key] {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("read Google keys: %w", err)
	}
	return json.NewEncoder(os.Stdout).Encode(keys)
}
