package controllers

import (
	"context"
	"encoding/json"
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

type APIKeyController struct {
	DB db.DBInterface
}

type APIKeyStats struct {
	TotalKeys       int            `json:"total_keys"`
	ValidKeys       int            `json:"valid_keys"`
	InvalidKeys     int            `json:"invalid_keys"`
	PendingKeys     int            `json:"pending_keys"`
	ErrorKeys       int            `json:"error_keys"`
	ByProvider      map[string]int `json:"by_provider"`
	LastScrapedAt   *time.Time     `json:"last_scraped_at,omitempty"`
	LastValidatedAt *time.Time     `json:"last_validated_at,omitempty"`
}

const (
	activeValidProviderCountsCacheKey = "api_keys:active_valid_provider_counts"
	activeValidProviderCountsCacheTTL = 5 * time.Hour
)

func (c *APIKeyController) GetCollectionName() string {
	return "api_keys"
}

// PerformIndexing creates MongoDB indexes for the api_keys collection
func (c *APIKeyController) PerformIndexing() error {
	if c.DB == nil {
		log.Println("Warning: DB instance is nil, skipping indexing")
		return nil
	}

	// Create unique index on key_value
	uniqueIndex := bson.D{{Key: "key_value", Value: 1}}
	if err := c.DB.ValidateUniqueIndexing(c.GetCollectionName(), uniqueIndex); err != nil {
		log.Println("Error creating unique index on key_value:", err)
		return err
	}

	// Create indexes on other fields
	indexes := []bson.D{
		{{Key: "status", Value: 1}},
		{{Key: "provider", Value: 1}},
		{{Key: "created_at", Value: -1}},
		{{Key: "validated_at", Value: -1}},
		{{Key: "status", Value: 1}, {Key: "provider", Value: 1}, {Key: "created_at", Value: -1}},
	}

	for _, index := range indexes {
		if err := c.DB.ValidateIndexing(c.GetCollectionName(), index); err != nil {
			log.Println("Error creating index:", err)
			return err
		}
	}

	// This ensures no duplicate references for the same file path, repo URL, and API key
	repoRefUniqueIndex := bson.D{
		{Key: "api_key_id", Value: 1},
		{Key: "file_path", Value: 1},
		{Key: "repo_url", Value: 1},
	}
	if err := c.DB.ValidateUniqueIndexing("repo_references", repoRefUniqueIndex); err != nil {
		log.Println("Error creating unique compound index on repo_references:", err)
		return err
	}

	// Create additional indexes on repo_references for query performance
	repoRefIndexes := []bson.D{
		{{Key: "api_key_id", Value: 1}},
		{{Key: "found_at", Value: -1}},
	}

	for _, index := range repoRefIndexes {
		if err := c.DB.ValidateIndexing("repo_references", index); err != nil {
			log.Println("Error creating index on repo_references:", err)
			return err
		}
	}

	return nil
}

// Create inserts a new API key into the database
func (c *APIKeyController) Create(key *model.APIKey) (primitive.ObjectID, error) {
	result, err := c.DB.Create(key, c.GetCollectionName())
	if err != nil {
		return primitive.NilObjectID, err
	}
	if key.Status == model.StatusValid || key.Status == model.StatusValidNoCredits {
		c.invalidateActiveValidProviderCountsCache()
	}

	if id, ok := result["_id"].(primitive.ObjectID); ok {
		return id, nil
	}

	return primitive.NilObjectID, nil
}

// FindOne retrieves a single API key matching the query
func (c *APIKeyController) FindOne(query bson.M) (*model.APIKey, error) {
	result, err := c.DB.FindOne(query, c.GetCollectionName())
	if err != nil {
		return nil, err
	}

	var apiKey model.APIKey
	bsonBytes, _ := bson.Marshal(result)
	if err := bson.Unmarshal(bsonBytes, &apiKey); err != nil {
		return nil, err
	}

	return &apiKey, nil
}

// FindByKeyValue retrieves an API key by its key value
func (c *APIKeyController) FindByKeyValue(keyValue string) (*model.APIKey, error) {
	query := bson.M{"key_value": keyValue}
	return c.FindOne(query)
}

func (c *APIKeyController) FindValidByProvider(provider string) (*model.APIKey, error) {
	return c.FindOne(bson.M{
		"provider": provider,
		"status":   model.StatusValid,
	})
}

func (c *APIKeyController) FindAllValidByProvider(provider string) ([]*model.APIKey, error) {
	return c.findAllByQuery(bson.M{
		"provider": provider,
		"status":   model.StatusValid,
	})
}

// FindByStatus retrieves all API keys with a specific status
func (c *APIKeyController) FindByStatus(status string) ([]*model.APIKey, error) {
	return c.findAllByQuery(bson.M{"status": status})
}

func (c *APIKeyController) findAllByQuery(query bson.M) ([]*model.APIKey, error) {
	totalPages, _, results, err := c.DB.FindAllWithPagination(query, 1, c.GetCollectionName())
	if err != nil {
		return nil, err
	}

	var apiKeys []*model.APIKey
	appendResults := func(pageResults []bson.M) {
		for _, result := range pageResults {
			var apiKey model.APIKey
			bsonBytes, _ := bson.Marshal(result)
			if err := bson.Unmarshal(bsonBytes, &apiKey); err != nil {
				continue
			}
			apiKeys = append(apiKeys, &apiKey)
		}
	}

	appendResults(results)
	for page := 2; page <= int(totalPages); page++ {
		_, _, pageResults, err := c.DB.FindAllWithPagination(query, page, c.GetCollectionName())
		if err != nil {
			return nil, err
		}
		appendResults(pageResults)
	}

	return apiKeys, nil
}

// FindByStatusWithReferences retrieves all API keys with a specific status (repo references NOT populated for performance)
func (c *APIKeyController) FindByStatusWithReferences(status string) ([]*model.APIKeyWithReferences, error) {
	query := bson.M{"status": status}
	totalPages, _, results, err := c.DB.FindAllWithPagination(query, 1, c.GetCollectionName())
	if err != nil {
		return nil, err
	}

	var apiKeys []*model.APIKeyWithReferences
	appendResults := func(pageResults []bson.M) {
		for _, result := range pageResults {
			var apiKey model.APIKey
			bsonBytes, _ := bson.Marshal(result)
			if err := bson.Unmarshal(bsonBytes, &apiKey); err != nil {
				continue
			}

			apiKeyWithRefs := &model.APIKeyWithReferences{
				APIKey: apiKey,
			}
			apiKeys = append(apiKeys, apiKeyWithRefs)
		}
	}

	appendResults(results)
	for page := 2; page <= int(totalPages); page++ {
		_, _, pageResults, err := c.DB.FindAllWithPagination(query, page, c.GetCollectionName())
		if err != nil {
			return nil, err
		}
		appendResults(pageResults)
	}

	return apiKeys, nil
}

// FindPendingKeys retrieves all API keys with Pending status
func (c *APIKeyController) FindPendingKeys() ([]*model.APIKey, error) {
	return c.FindByStatus(model.StatusPending)
}

// FindAll retrieves all API keys from the database
func (c *APIKeyController) FindAll() ([]*model.APIKey, error) {
	query := bson.M{}
	totalPages, _, results, err := c.DB.FindAllWithPagination(query, 1, c.GetCollectionName())
	if err != nil {
		return nil, err
	}

	var apiKeys []*model.APIKey
	appendResults := func(pageResults []bson.M) {
		for _, result := range pageResults {
			var apiKey model.APIKey
			bsonBytes, _ := bson.Marshal(result)
			if err := bson.Unmarshal(bsonBytes, &apiKey); err != nil {
				continue
			}
			apiKeys = append(apiKeys, &apiKey)
		}
	}

	appendResults(results)
	for page := 2; page <= int(totalPages); page++ {
		_, _, pageResults, err := c.DB.FindAllWithPagination(query, page, c.GetCollectionName())
		if err != nil {
			return nil, err
		}
		appendResults(pageResults)
	}

	return apiKeys, nil
}

// FindAllWithPagination retrieves API keys with pagination
func (c *APIKeyController) FindAllWithPagination(query bson.M, page int) (int64, int, []*model.APIKey, error) {
	totalPages, currentPage, results, err := c.DB.FindAllWithPagination(query, page, c.GetCollectionName())
	if err != nil {
		return 0, 0, nil, err
	}

	var apiKeys []*model.APIKey
	for _, result := range results {
		var apiKey model.APIKey
		bsonBytes, _ := bson.Marshal(result)
		if err := bson.Unmarshal(bsonBytes, &apiKey); err != nil {
			continue
		}
		apiKeys = append(apiKeys, &apiKey)
	}

	return totalPages, currentPage, apiKeys, nil
}

// FindAllWithPaginationAndReferences retrieves API keys with pagination (repo references NOT populated for performance)
func (c *APIKeyController) FindAllWithPaginationAndReferences(query bson.M, page int) (int64, int, []*model.APIKeyWithReferences, error) {
	// Use direct MongoDB access for sorting support
	dbConn := db.GetConnectionFromPool()
	defer db.ReleaseConnectionToPool(dbConn)

	collection := dbConn.Client.Database(os.Getenv("MONGO_DB_NAME")).Collection(c.GetCollectionName())
	ctx := context.Background()

	const pageSize = 10
	if page < 1 {
		page = 1
	}

	// Calculate total documents
	totalDocs, err := collection.CountDocuments(ctx, query)
	if err != nil {
		return 0, 0, nil, err
	}

	// Calculate total pages
	totalPages := totalDocs / pageSize
	if totalDocs%pageSize > 0 {
		totalPages++
	}

	// Fetch documents with pagination and sorting (descending by created_at)
	opts := options.Find()
	opts.SetLimit(pageSize)
	opts.SetSkip(pageSize * int64(page-1))
	opts.SetSort(bson.D{{Key: "created_at", Value: -1}}) // Sort by created_at descending

	cursor, err := collection.Find(ctx, query, opts)
	if err != nil {
		return 0, 0, nil, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return 0, 0, nil, err
	}

	// Build results without fetching repo references for better performance
	var apiKeys []*model.APIKeyWithReferences
	for _, result := range results {
		var apiKey model.APIKey
		bsonBytes, _ := bson.Marshal(result)
		if err := bson.Unmarshal(bsonBytes, &apiKey); err != nil {
			continue
		}

		apiKeyWithRefs := &model.APIKeyWithReferences{
			APIKey: apiKey,
		}
		apiKeys = append(apiKeys, apiKeyWithRefs)
	}

	return totalPages, page, apiKeys, nil
}

// FindRepoReferencesByKeyID fetches all repo references linked to a specific API key
func (c *APIKeyController) FindRepoReferencesByKeyID(keyID primitive.ObjectID) ([]*model.RepoReference, error) {
	dbConn := db.GetConnectionFromPool()
	defer db.ReleaseConnectionToPool(dbConn)

	collection := dbConn.Client.Database(os.Getenv("MONGO_DB_NAME")).Collection("repo_references")
	ctx := context.Background()

	cursor, err := collection.Find(ctx, bson.M{"api_key_id": keyID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var refs []bson.M
	if err = cursor.All(ctx, &refs); err != nil {
		return nil, err
	}

	var references []*model.RepoReference
	for _, refResult := range refs {
		var ref model.RepoReference
		refBytes, _ := bson.Marshal(refResult)
		if err := bson.Unmarshal(refBytes, &ref); err != nil {
			continue
		}
		references = append(references, &ref)
	}

	return references, nil
}

// Update updates an API key by ID
func (c *APIKeyController) Update(id primitive.ObjectID, update bson.M) error {
	query := bson.M{"_id": id}
	_, err := c.DB.Update(query, update, c.GetCollectionName())
	if err == nil {
		if _, hasStatus := update["status"]; hasStatus {
			c.invalidateActiveValidProviderCountsCache()
		}
	}
	return err
}

// UpdateStatus updates the status of an API key
func (c *APIKeyController) UpdateStatus(id primitive.ObjectID, status string) error {
	now := time.Now()
	update := bson.M{
		"status":       status,
		"validated_at": now,
	}
	if err := c.Update(id, update); err != nil {
		return err
	}
	return nil
}

// UpdateStatusAndCredits updates the status and credits information of an API key
func (c *APIKeyController) UpdateStatusAndCredits(id primitive.ObjectID, status string, credits map[string]interface{}) error {
	now := time.Now()
	update := bson.M{
		"status":       status,
		"validated_at": now,
	}

	// Only add credits if provided
	if credits != nil {
		update["credits"] = credits
	}

	if err := c.Update(id, update); err != nil {
		return err
	}
	return nil
}

// GetActiveValidProviderCounts returns the number of active valid keys for each
// provider. The aggregate is cached because it is used by the stats endpoint
// and does not need to be recalculated on every request.
func (c *APIKeyController) GetActiveValidProviderCounts() (map[string]int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if redisCache := cache.GetInstance(); redisCache != nil {
		if client, ok := redisCache.GetClient().(*redis.Client); ok && client != nil {
			cached, err := client.Get(ctx, activeValidProviderCountsCacheKey).Result()
			if err == nil {
				var counts map[string]int
				if unmarshalErr := json.Unmarshal([]byte(cached), &counts); unmarshalErr == nil {
					return counts, nil
				}
			} else if err != redis.Nil {
				log.Printf("Warning: Failed to read active valid provider counts from Redis: %v", err)
			}
		}
	}

	dbConn := db.GetConnectionFromPool()
	defer db.ReleaseConnectionToPool(dbConn)

	collection := dbConn.Client.Database(os.Getenv("MONGO_DB_NAME")).Collection(c.GetCollectionName())
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{
			"status": bson.M{"$in": []string{model.StatusValid, model.StatusValidNoCredits}},
		}}},
		bson.D{{Key: "$group", Value: bson.M{"_id": "$provider", "count": bson.M{"$sum": 1}}}},
	}
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	counts := make(map[string]int)
	for cursor.Next(ctx) {
		var result struct {
			Provider string `bson:"_id"`
			Count    int    `bson:"count"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
		counts[result.Provider] = result.Count
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if redisCache := cache.GetInstance(); redisCache != nil {
		if client, ok := redisCache.GetClient().(*redis.Client); ok && client != nil {
			if encoded, marshalErr := json.Marshal(counts); marshalErr == nil {
				if err := client.Set(ctx, activeValidProviderCountsCacheKey, encoded, activeValidProviderCountsCacheTTL).Err(); err != nil {
					log.Printf("Warning: Failed to cache active valid provider counts in Redis: %v", err)
				}
			}
		}
	}

	return counts, nil
}

func (c *APIKeyController) invalidateActiveValidProviderCountsCache() {
	redisCache := cache.GetInstance()
	if redisCache == nil {
		return
	}
	client, ok := redisCache.GetClient().(*redis.Client)
	if !ok || client == nil {
		return
	}
	if err := client.Del(context.Background(), activeValidProviderCountsCacheKey).Err(); err != nil {
		log.Printf("Warning: Failed to invalidate active valid provider counts cache: %v", err)
	}
}

// UpdateNotifiedAt updates the notified_at timestamp for a key
func (c *APIKeyController) UpdateNotifiedAt(id primitive.ObjectID) error {
	now := time.Now()
	update := bson.M{
		"notified_at": now,
	}
	return c.Update(id, update)
}

// UpdateLastSeen updates the last_seen_at timestamp for a key
func (c *APIKeyController) UpdateLastSeen(keyValue string) error {
	query := bson.M{"key_value": keyValue}
	update := bson.M{"last_seen_at": time.Now()}
	_, err := c.DB.Update(query, update, c.GetCollectionName())
	return err
}

// UpsertByKeyValue creates a new API key or updates last_seen_at if it already exists
// Returns the key ID and a boolean indicating if it was newly created
func (c *APIKeyController) UpsertByKeyValue(key *model.APIKey) (primitive.ObjectID, bool, error) {
	// First check if key exists
	existingKey, err := c.FindByKeyValue(key.KeyValue)
	isNew := err != nil // If error (not found), it's new

	if isNew {
		// Create new key
		id, err := c.Create(key)
		if err != nil {
			// Check if it's a race condition duplicate
			if mongo.IsDuplicateKeyError(err) {
				// Another goroutine created it, fetch and return
				existingKey, fetchErr := c.FindByKeyValue(key.KeyValue)
				if fetchErr != nil {
					return primitive.NilObjectID, false, fetchErr
				}
				// Update last_seen_at
				_ = c.UpdateLastSeen(key.KeyValue)
				return existingKey.ID, false, nil
			}
			return primitive.NilObjectID, false, err
		}
		return id, true, nil
	}

	// Update existing key's last_seen_at
	if err := c.UpdateLastSeen(key.KeyValue); err != nil {
		return primitive.NilObjectID, false, err
	}

	return existingKey.ID, false, nil
}

// AddRepoReference creates a repository reference and adds its ID to an API key's repo_refs array
// Only creates a new reference if one doesn't already exist for the same file path and API key
func (c *APIKeyController) AddRepoReference(keyID primitive.ObjectID, ref *model.RepoReference) error {
	// Check if this reference already exists for this API key
	existingRefQuery := bson.M{
		"api_key_id": keyID,
		"file_path":  ref.FilePath,
		"repo_url":   ref.RepoURL,
	}

	existingRef, err := c.DB.FindOne(existingRefQuery, "repo_references")
	if err == nil && existingRef != nil {
		// Reference already exists, check if it's already in the API key's repo_refs array
		var existingRefID primitive.ObjectID
		if id, ok := existingRef["_id"].(primitive.ObjectID); ok {
			existingRefID = id
		} else {
			return nil
		}

		// Use $addToSet to add the reference ID only if it doesn't already exist
		// This is atomic and prevents duplicates even with concurrent calls
		keyQuery := bson.M{"_id": keyID}
		update := bson.M{
			"$addToSet": bson.M{
				"repo_refs": existingRefID,
			},
		}
		_, err = c.DB.UpdateWithOperators(keyQuery, update, c.GetCollectionName())
		return err
	}

	// Create the repository reference (it doesn't exist yet)
	result, err := c.DB.Create(ref, "repo_references")
	if err != nil {
		// Check if it's a duplicate key error (race condition)
		if mongo.IsDuplicateKeyError(err) {
			// Another goroutine created it, fetch and add to array
			existingRef, fetchErr := c.DB.FindOne(existingRefQuery, "repo_references")
			if fetchErr != nil {
				return fetchErr
			}

			var existingRefID primitive.ObjectID
			if id, ok := existingRef["_id"].(primitive.ObjectID); ok {
				existingRefID = id
			} else {
				return nil
			}

			// Use $addToSet to add the reference ID atomically
			keyQuery := bson.M{"_id": keyID}
			update := bson.M{
				"$addToSet": bson.M{
					"repo_refs": existingRefID,
				},
			}
			_, err = c.DB.UpdateWithOperators(keyQuery, update, c.GetCollectionName())
			return err
		}
		return err
	}

	var refID primitive.ObjectID
	if id, ok := result["_id"].(primitive.ObjectID); ok {
		refID = id
	} else {
		return nil
	}

	// Use $addToSet to add the new reference ID atomically
	// This prevents duplicates even if called concurrently
	query := bson.M{"_id": keyID}
	update := bson.M{
		"$addToSet": bson.M{
			"repo_refs": refID,
		},
	}
	_, err = c.DB.UpdateWithOperators(query, update, c.GetCollectionName())
	return err
}

// GetStatistics returns aggregated statistics about API keys
func (c *APIKeyController) GetStatistics() (*APIKeyStats, error) {
	stats := &APIKeyStats{
		ByProvider: make(map[string]int),
	}
	if providerCounts, err := c.GetActiveValidProviderCounts(); err == nil {
		stats.ByProvider = providerCounts
	} else {
		log.Printf("Warning: Failed to get active valid provider counts: %v", err)
	}

	dbConn := db.GetConnectionFromPool()
	defer db.ReleaseConnectionToPool(dbConn)

	collection := dbConn.Client.Database(os.Getenv("MONGO_DB_NAME")).Collection(c.GetCollectionName())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Return only the counters and two timestamps instead of loading every key.
	pipeline := mongo.Pipeline{bson.D{{Key: "$group", Value: bson.M{
		"_id":            nil,
		"total":          bson.M{"$sum": 1},
		"valid":          bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$in": []interface{}{"$status", []string{model.StatusValid, model.StatusValidNoCredits}}}, 1, 0}}},
		"invalid":        bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []interface{}{"$status", model.StatusInvalid}}, 1, 0}}},
		"pending":        bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []interface{}{"$status", model.StatusPending}}, 1, 0}}},
		"errors":         bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []interface{}{"$status", model.StatusError}}, 1, 0}}},
		"last_validated": bson.M{"$max": "$validated_at"},
		"last_created":   bson.M{"$max": "$created_at"},
	}}}}
	var aggregate struct {
		Total         int        `bson:"total"`
		Valid         int        `bson:"valid"`
		Invalid       int        `bson:"invalid"`
		Pending       int        `bson:"pending"`
		Errors        int        `bson:"errors"`
		LastValidated *time.Time `bson:"last_validated"`
		LastCreated   *time.Time `bson:"last_created"`
	}
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if cursor.Next(ctx) {
		if err := cursor.Decode(&aggregate); err != nil {
			return nil, err
		}
	}
	if err = cursor.Err(); err != nil {
		return nil, err
	}
	stats.TotalKeys = aggregate.Total
	stats.ValidKeys = aggregate.Valid
	stats.InvalidKeys = aggregate.Invalid
	stats.PendingKeys = aggregate.Pending
	stats.ErrorKeys = aggregate.Errors
	stats.LastValidatedAt = aggregate.LastValidated

	// Use the persisted last_scraped_at from scraper metadata instead of deriving from key CreatedAt
	if lastScrapedAt, err := c.GetLastScrapedAt(); err == nil && lastScrapedAt != nil {
		stats.LastScrapedAt = lastScrapedAt
	} else {
		// Fallback to derived value if metadata not yet available
		stats.LastScrapedAt = aggregate.LastCreated
	}

	return stats, nil
}

const scraperMetadataCollection = "scraper_metadata"
const scraperMetadataKey = "global"

// UpdateLastScrapedAt persists the current scrape start time to the database.
// This is called at the start of each scraping cycle so that the stats endpoint
// reflects when scraping is actively running, not just when keys were last discovered.
func (c *APIKeyController) UpdateLastScrapedAt(t time.Time) error {
	if c.DB == nil {
		return nil
	}
	query := bson.M{"key": scraperMetadataKey}
	update := bson.M{
		"key":             scraperMetadataKey,
		"last_scraped_at": t,
	}
	c.DB.UpdateOrCreate(query, update, scraperMetadataCollection)
	return nil
}

// GetLastScrapedAt retrieves the persisted last scrape time from the database.
func (c *APIKeyController) GetLastScrapedAt() (*time.Time, error) {
	if c.DB == nil {
		return nil, nil
	}
	query := bson.M{"key": scraperMetadataKey}
	result, err := c.DB.FindOne(query, scraperMetadataCollection)
	if err != nil || result == nil {
		return nil, err
	}
	if raw, ok := result["last_scraped_at"]; ok {
		if t, ok := raw.(primitive.DateTime); ok {
			tt := t.Time()
			return &tt, nil
		}
	}
	return nil, nil
}

// DeleteOldestValidKeys deletes the oldest valid keys to enforce the limit
func (c *APIKeyController) DeleteOldestValidKeys(keepCount int) error {
	// Get all valid keys
	validKeys, err := c.FindByStatus(model.StatusValid)
	if err != nil {
		return err
	}

	if len(validKeys) <= keepCount {
		return nil
	}

	// Sort by validated_at (oldest first)
	// Note: FindByStatus doesn't guarantee order, so we need to sort manually
	// For simplicity, we'll delete the excess keys
	deleteCount := len(validKeys) - keepCount

	// Delete the oldest keys
	for i := 0; i < deleteCount; i++ {
		query := bson.M{"_id": validKeys[i].ID}
		_, err := c.DB.Delete(query, c.GetCollectionName())
		if err != nil {
			log.Printf("Error deleting key %s: %v", validKeys[i].ID.Hex(), err)
			continue
		}
	}

	c.invalidateActiveValidProviderCountsCache()
	log.Printf("Deleted %d oldest valid keys to enforce limit of %d", deleteCount, keepCount)
	return nil
}
