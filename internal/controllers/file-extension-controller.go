package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"project-phoenix/v2/internal/cache"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/model"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// Redis keys for file extension caching
	redisKeyNextExtension = "file_extension:next"
	redisKeyAllExtensions = "file_extension:all"
	redisTTLHours         = 1 // Cache for 1 hour
)

// FileExtensionController handles file extension operations with Redis caching
type FileExtensionController struct {
	DB          db.DBInterface
	redisClient *cache.Redis
}

func (c *FileExtensionController) GetCollectionName() string {
	return "file_extensions"
}

// getMongoCollection returns the MongoDB collection for file extensions
func (c *FileExtensionController) getMongoCollection() (*mongo.Collection, error) {
	mongoInstance, ok := c.DB.(*db.MongoDB)
	if !ok {
		return nil, fmt.Errorf("database is not MongoDB")
	}

	// Get database name from environment
	dbName := os.Getenv("MONGO_DB_NAME")
	if dbName == "" {
		dbName = "project_phoenix" // Default database name
	}

	return mongoInstance.Client.Database(dbName).Collection(c.GetCollectionName()), nil
}

// InitializeDefaults creates default file extensions if they don't exist
// Also seeds Redis cache after DB initialization
func (c *FileExtensionController) InitializeDefaults() error {
	for _, ext := range model.DefaultFileExtensions {
		filter := bson.M{"extension": ext.Extension}

		// Check if extension already exists
		existing, err := c.DB.FindOne(filter, c.GetCollectionName())
		if err == nil && existing != nil {
			continue // Extension already exists
		}

		// Create new extension
		fileExt := &model.FileExtension{
			Extension:      ext.Extension,
			Priority:       ext.Priority,
			Enabled:        true, // Enable by default
			LastSearchedAt: nil,
			ResultCount:    0,
			KeysFound:      0,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		if _, err := c.DB.Create(fileExt, c.GetCollectionName()); err != nil {
			log.Printf("Failed to insert extension %s: %v", ext.Extension, err)
			return fmt.Errorf("failed to insert extension %s: %w", ext.Extension, err)
		}
	}

	// Seed Redis cache after DB initialization
	if err := c.seedRedisCache(); err != nil {
		log.Printf("Warning: Failed to seed Redis cache: %v", err)
		// Don't fail initialization if Redis seeding fails
	}

	return nil
}

// seedRedisCache loads all extensions from DB into Redis
func (c *FileExtensionController) seedRedisCache() error {
	if c.redisClient == nil {
		return nil // Redis disabled, skip
	}

	extensions, err := c.getAllExtensionsFromDB()
	if err != nil {
		return fmt.Errorf("failed to load extensions from DB: %w", err)
	}

	// Store all extensions in Redis
	extensionsJSON, err := json.Marshal(extensions)
	if err != nil {
		return fmt.Errorf("failed to marshal extensions: %w", err)
	}

	ctx := context.Background()
	ttl := time.Hour * time.Duration(redisTTLHours)
	if err := c.redisClient.GetClient().(*redis.Client).Set(ctx, redisKeyAllExtensions, extensionsJSON, ttl).Err(); err != nil {
		return fmt.Errorf("failed to cache extensions in Redis: %w", err)
	}

	log.Printf("Seeded %d file extensions to Redis cache", len(extensions))
	return nil
}

// GetNextExtensionToSearch returns the next file extension that should be searched
// Uses Redis cache for fast lookups, falls back to MongoDB if cache miss
// Selection logic:
// 1. Enabled extensions only
// 2. Never searched before (LastSearchedAt is nil) - prioritized by Priority field
// 3. Least recently searched - prioritized by Priority field
func (c *FileExtensionController) GetNextExtensionToSearch() (*model.FileExtension, error) {
	// Try Redis cache first
	if c.redisClient != nil {
		if ext, err := c.getNextExtensionFromRedis(); err == nil && ext != nil {
			return ext, nil
		}
		// Cache miss or error, fall through to DB
	}

	// Fetch from MongoDB
	ext, err := c.getNextExtensionFromDB()
	if err != nil {
		return nil, err
	}

	// Update Redis cache with the result
	if c.redisClient != nil && ext != nil {
		if err := c.cacheNextExtension(ext); err != nil {
			log.Printf("Warning: Failed to cache next extension in Redis: %v", err)
		}
	}

	return ext, nil
}

// getNextExtensionFromRedis attempts to fetch the next extension from Redis cache
func (c *FileExtensionController) getNextExtensionFromRedis() (*model.FileExtension, error) {
	ctx := context.Background()
	data, err := c.redisClient.GetClient().(*redis.Client).Get(ctx, redisKeyNextExtension).Result()
	if err != nil {
		return nil, err // Cache miss
	}

	var extension model.FileExtension
	if err := json.Unmarshal([]byte(data), &extension); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached extension: %w", err)
	}

	return &extension, nil
}

// getNextExtensionFromDB fetches the next extension from MongoDB
func (c *FileExtensionController) getNextExtensionFromDB() (*model.FileExtension, error) {
	collection, err := c.getMongoCollection()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First, try to find extensions that have never been searched (highest priority)
	filter := bson.M{
		"enabled":          true,
		"last_searched_at": nil,
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "priority", Value: 1}}) // Sort by priority ascending (1 is highest)

	var extension model.FileExtension
	err = collection.FindOne(ctx, filter, opts).Decode(&extension)
	if err == nil {
		return &extension, nil
	}

	if err != mongo.ErrNoDocuments {
		return nil, fmt.Errorf("failed to find never-searched extension: %w", err)
	}

	// If all extensions have been searched, find the least recently searched one
	filter = bson.M{"enabled": true}
	opts = options.FindOne().SetSort(bson.D{
		{Key: "last_searched_at", Value: 1}, // Oldest first
		{Key: "priority", Value: 1},         // Then by priority
	})

	err = collection.FindOne(ctx, filter, opts).Decode(&extension)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("no enabled extensions found")
		}
		return nil, fmt.Errorf("failed to find least recently searched extension: %w", err)
	}

	return &extension, nil
}

// cacheNextExtension stores the next extension in Redis cache
func (c *FileExtensionController) cacheNextExtension(ext *model.FileExtension) error {
	extensionJSON, err := json.Marshal(ext)
	if err != nil {
		return fmt.Errorf("failed to marshal extension: %w", err)
	}

	ctx := context.Background()
	ttl := time.Minute * 5 // Short TTL since this changes frequently
	if err := c.redisClient.GetClient().(*redis.Client).Set(ctx, redisKeyNextExtension, extensionJSON, ttl).Err(); err != nil {
		return fmt.Errorf("failed to cache next extension: %w", err)
	}

	return nil
}

// UpdateSearchStats updates the search statistics for a file extension
// Uses write-through cache pattern: updates both DB and Redis
func (c *FileExtensionController) UpdateSearchStats(id primitive.ObjectID, resultCount int, keysFound int) error {
	now := time.Now()
	update := bson.M{
		"last_searched_at": now,
		"result_count":     resultCount,
		"keys_found":       keysFound,
		"updated_at":       now,
	}

	filter := bson.M{"_id": id}

	// Update MongoDB
	_, err := c.DB.Update(filter, update, c.GetCollectionName())
	if err != nil {
		return fmt.Errorf("failed to update search stats: %w", err)
	}

	// Invalidate Redis cache (write-through pattern)
	if c.redisClient != nil {
		ctx := context.Background()
		// Delete cached next extension since stats changed
		c.redisClient.GetClient().(*redis.Client).Del(ctx, redisKeyNextExtension)
		// Delete cached all extensions list
		c.redisClient.GetClient().(*redis.Client).Del(ctx, redisKeyAllExtensions)

		log.Printf("Invalidated Redis cache after updating extension stats")
	}

	return nil
}

// getAllExtensionsFromDB fetches all extensions directly from MongoDB (bypasses cache)
func (c *FileExtensionController) getAllExtensionsFromDB() ([]*model.FileExtension, error) {
	collection, err := c.getMongoCollection()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "priority", Value: 1}})
	cursor, err := collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find extensions: %w", err)
	}
	defer cursor.Close(ctx)

	var extensions []*model.FileExtension
	if err := cursor.All(ctx, &extensions); err != nil {
		return nil, fmt.Errorf("failed to decode extensions: %w", err)
	}

	return extensions, nil
}

// GetAllExtensions returns all file extensions
// Uses Redis cache for fast lookups, falls back to MongoDB if cache miss
func (c *FileExtensionController) GetAllExtensions() ([]*model.FileExtension, error) {
	// Try Redis cache first
	if c.redisClient != nil {
		if extensions, err := c.getAllExtensionsFromRedis(); err == nil && extensions != nil {
			return extensions, nil
		}
		// Cache miss or error, fall through to DB
	}

	// Fetch from MongoDB
	extensions, err := c.getAllExtensionsFromDB()
	if err != nil {
		return nil, err
	}

	// Update Redis cache
	if c.redisClient != nil && len(extensions) > 0 {
		if err := c.cacheAllExtensions(extensions); err != nil {
			log.Printf("Warning: Failed to cache all extensions in Redis: %v", err)
		}
	}

	return extensions, nil
}

// getAllExtensionsFromRedis attempts to fetch all extensions from Redis cache
func (c *FileExtensionController) getAllExtensionsFromRedis() ([]*model.FileExtension, error) {
	ctx := context.Background()
	data, err := c.redisClient.GetClient().(*redis.Client).Get(ctx, redisKeyAllExtensions).Result()
	if err != nil {
		return nil, err // Cache miss
	}

	var extensions []*model.FileExtension
	if err := json.Unmarshal([]byte(data), &extensions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached extensions: %w", err)
	}

	return extensions, nil
}

// cacheAllExtensions stores all extensions in Redis cache
func (c *FileExtensionController) cacheAllExtensions(extensions []*model.FileExtension) error {
	extensionsJSON, err := json.Marshal(extensions)
	if err != nil {
		return fmt.Errorf("failed to marshal extensions: %w", err)
	}

	ctx := context.Background()
	ttl := time.Hour * time.Duration(redisTTLHours)
	if err := c.redisClient.GetClient().(*redis.Client).Set(ctx, redisKeyAllExtensions, extensionsJSON, ttl).Err(); err != nil {
		return fmt.Errorf("failed to cache extensions: %w", err)
	}

	return nil
}

// GetEnabledExtensions returns all enabled file extensions
func (c *FileExtensionController) GetEnabledExtensions() ([]*model.FileExtension, error) {
	collection, err := c.getMongoCollection()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"enabled": true}
	opts := options.Find().SetSort(bson.D{{Key: "priority", Value: 1}})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find enabled extensions: %w", err)
	}
	defer cursor.Close(ctx)

	var extensions []*model.FileExtension
	if err := cursor.All(ctx, &extensions); err != nil {
		return nil, fmt.Errorf("failed to decode extensions: %w", err)
	}

	return extensions, nil
}

// UpdateExtension updates a file extension
// Uses write-through cache pattern: updates both DB and invalidates Redis cache
func (c *FileExtensionController) UpdateExtension(id primitive.ObjectID, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	filter := bson.M{"_id": id}

	_, err := c.DB.Update(filter, updates, c.GetCollectionName())
	if err != nil {
		return fmt.Errorf("failed to update extension: %w", err)
	}

	// Invalidate Redis cache (write-through pattern)
	if c.redisClient != nil {
		ctx := context.Background()
		c.redisClient.GetClient().(*redis.Client).Del(ctx, redisKeyNextExtension)
		c.redisClient.GetClient().(*redis.Client).Del(ctx, redisKeyAllExtensions)
		log.Printf("Invalidated Redis cache after updating extension")
	}

	return nil
}

// EnableExtension enables a file extension
func (c *FileExtensionController) EnableExtension(id primitive.ObjectID) error {
	return c.UpdateExtension(id, map[string]interface{}{"enabled": true})
}

// DisableExtension disables a file extension
func (c *FileExtensionController) DisableExtension(id primitive.ObjectID) error {
	return c.UpdateExtension(id, map[string]interface{}{"enabled": false})
}
