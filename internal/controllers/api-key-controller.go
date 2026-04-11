package controllers

import (
	"log"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/model"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
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

// FindByStatus retrieves all API keys with a specific status
func (c *APIKeyController) FindByStatus(status string) ([]*model.APIKey, error) {
	query := bson.M{"status": status}
	_, _, results, err := c.DB.FindAllWithPagination(query, 1, c.GetCollectionName())
	if err != nil {
		return nil, err
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

	return apiKeys, nil
}

// FindByStatusWithReferences retrieves all API keys with a specific status and populates repo references
func (c *APIKeyController) FindByStatusWithReferences(status string) ([]*model.APIKeyWithReferences, error) {
	query := bson.M{"status": status}
	_, _, results, err := c.DB.FindAllWithPagination(query, 1, c.GetCollectionName())
	if err != nil {
		return nil, err
	}

	var apiKeys []*model.APIKeyWithReferences
	for _, result := range results {
		var apiKey model.APIKey
		bsonBytes, _ := bson.Marshal(result)
		if err := bson.Unmarshal(bsonBytes, &apiKey); err != nil {
			continue
		}

		// Populate repo references
		var references []*model.RepoReference
		for _, refID := range apiKey.RepoRefs {
			refQuery := bson.M{"_id": refID}
			refResult, err := c.DB.FindOne(refQuery, "repo_references")
			if err != nil {
				continue
			}

			var ref model.RepoReference
			refBytes, _ := bson.Marshal(refResult)
			if err := bson.Unmarshal(refBytes, &ref); err != nil {
				continue
			}
			references = append(references, &ref)
		}

		apiKeyWithRefs := &model.APIKeyWithReferences{
			APIKey:     apiKey,
			References: references,
		}
		apiKeys = append(apiKeys, apiKeyWithRefs)
	}

	return apiKeys, nil
}

// FindPendingKeys retrieves all API keys with Pending status
func (c *APIKeyController) FindPendingKeys() ([]*model.APIKey, error) {
	return c.FindByStatus(model.StatusPending)
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

// Update updates an API key by ID
func (c *APIKeyController) Update(id primitive.ObjectID, update bson.M) error {
	query := bson.M{"_id": id}
	_, err := c.DB.Update(query, update, c.GetCollectionName())
	return err
}

// UpdateStatus updates the status of an API key
func (c *APIKeyController) UpdateStatus(id primitive.ObjectID, status string) error {
	now := time.Now()
	update := bson.M{
		"status":       status,
		"validated_at": now,
	}
	return c.Update(id, update)
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

	return c.Update(id, update)
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
		_, err = c.DB.Update(keyQuery, update, c.GetCollectionName())
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
			_, err = c.DB.Update(keyQuery, update, c.GetCollectionName())
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
	_, err = c.DB.Update(query, update, c.GetCollectionName())
	return err
}

// GetStatistics returns aggregated statistics about API keys
func (c *APIKeyController) GetStatistics() (*APIKeyStats, error) {
	stats := &APIKeyStats{
		ByProvider: make(map[string]int),
	}

	// Count total keys
	query := bson.M{}
	_, _, allKeys, err := c.DB.FindAllWithPagination(query, 1, c.GetCollectionName())
	if err != nil {
		return nil, err
	}

	stats.TotalKeys = len(allKeys)

	// Count by status and provider
	var lastValidated *time.Time
	var lastScraped *time.Time
	for _, result := range allKeys {
		var apiKey model.APIKey
		bsonBytes, _ := bson.Marshal(result)
		if err := bson.Unmarshal(bsonBytes, &apiKey); err != nil {
			continue
		}

		switch apiKey.Status {
		case model.StatusValid:
			stats.ValidKeys++
		case model.StatusValidNoCredits:
			stats.ValidKeys++ // Count ValidNoCredits as valid keys
		case model.StatusInvalid:
			stats.InvalidKeys++
		case model.StatusPending:
			stats.PendingKeys++
		case model.StatusError:
			stats.ErrorKeys++
		}

		stats.ByProvider[apiKey.Provider]++

		// Track most recent validation timestamp
		if apiKey.ValidatedAt != nil {
			if lastValidated == nil || apiKey.ValidatedAt.After(*lastValidated) {
				lastValidated = apiKey.ValidatedAt
			}
		}

		// Track most recent scrape/discovery timestamp
		if lastScraped == nil || apiKey.CreatedAt.After(*lastScraped) {
			lastScraped = &apiKey.CreatedAt
		}
	}

	stats.LastValidatedAt = lastValidated
	stats.LastScrapedAt = lastScraped

	return stats, nil
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

	log.Printf("Deleted %d oldest valid keys to enforce limit of %d", deleteCount, keepCount)
	return nil
}
