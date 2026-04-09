package controllers

import (
	"log"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/model"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ScraperConfigController struct {
	DB db.DBInterface
}

func (c *ScraperConfigController) GetCollectionName() string {
	return "search_queries"
}

// PerformIndexing creates MongoDB indexes for the search_queries collection
func (c *ScraperConfigController) PerformIndexing() error {
	if c.DB == nil {
		log.Println("Warning: DB instance is nil, skipping indexing")
		return nil
	}

	indexes := []bson.D{
		{{Key: "enabled", Value: 1}},
		{{Key: "provider", Value: 1}},
	}

	for _, index := range indexes {
		if err := c.DB.ValidateIndexing(c.GetCollectionName(), index); err != nil {
			log.Println("Error creating index:", err)
			return err
		}
	}

	return nil
}

// CreateQuery inserts a new search query into the database
func (c *ScraperConfigController) CreateQuery(query *model.SearchQuery) (primitive.ObjectID, error) {
	if query.CreatedAt.IsZero() {
		query.CreatedAt = time.Now()
	}

	result, err := c.DB.Create(query, c.GetCollectionName())
	if err != nil {
		return primitive.NilObjectID, err
	}

	if id, ok := result["_id"].(primitive.ObjectID); ok {
		return id, nil
	}

	return primitive.NilObjectID, nil
}

// GetEnabledQueries retrieves all enabled search queries
func (c *ScraperConfigController) GetEnabledQueries() ([]*model.SearchQuery, error) {
	query := bson.M{"enabled": true}
	_, _, results, err := c.DB.FindAllWithPagination(query, 1, c.GetCollectionName())
	if err != nil {
		return nil, err
	}

	var queries []*model.SearchQuery
	for _, result := range results {
		var searchQuery model.SearchQuery
		bsonBytes, _ := bson.Marshal(result)
		if err := bson.Unmarshal(bsonBytes, &searchQuery); err != nil {
			continue
		}
		queries = append(queries, &searchQuery)
	}

	return queries, nil
}

// GetAllQueries retrieves all search queries
func (c *ScraperConfigController) GetAllQueries() ([]*model.SearchQuery, error) {
	query := bson.M{}
	_, _, results, err := c.DB.FindAllWithPagination(query, 1, c.GetCollectionName())
	if err != nil {
		return nil, err
	}

	var queries []*model.SearchQuery
	for _, result := range results {
		var searchQuery model.SearchQuery
		bsonBytes, _ := bson.Marshal(result)
		if err := bson.Unmarshal(bsonBytes, &searchQuery); err != nil {
			continue
		}
		queries = append(queries, &searchQuery)
	}

	return queries, nil
}

// UpdateQuery updates a search query by ID
func (c *ScraperConfigController) UpdateQuery(id primitive.ObjectID, update bson.M) error {
	query := bson.M{"_id": id}
	_, err := c.DB.Update(query, update, c.GetCollectionName())
	return err
}

// DeleteQuery deletes a search query by ID
func (c *ScraperConfigController) DeleteQuery(id primitive.ObjectID) error {
	query := bson.M{"_id": id}
	_, err := c.DB.Delete(query, c.GetCollectionName())
	return err
}

// UpdateQueryStats updates the statistics for a search query
func (c *ScraperConfigController) UpdateQueryStats(id primitive.ObjectID, resultCount int) error {
	now := time.Now()
	update := bson.M{
		"last_searched_at": now,
		"result_count":     resultCount,
	}
	return c.UpdateQuery(id, update)
}

// SeedDefaultQueries seeds the database with default search queries if they don't exist
func (c *ScraperConfigController) SeedDefaultQueries() error {
	defaultQueries := []model.SearchQuery{
		{
			QueryPattern: `"sk-" "openai" extension:env`,
			Provider:     model.ProviderOpenAI,
			Enabled:      true,
			CreatedAt:    time.Now(),
		},
		{
			QueryPattern: `"sk-ant-" extension:env`,
			Provider:     model.ProviderAnthropic,
			Enabled:      true,
			CreatedAt:    time.Now(),
		},
		{
			QueryPattern: `"AIza" "google" extension:env`,
			Provider:     model.ProviderGoogle,
			Enabled:      true,
			CreatedAt:    time.Now(),
		},
		{
			QueryPattern: `"sk-or-" extension:env`,
			Provider:     model.ProviderOpenRouter,
			Enabled:      true,
			CreatedAt:    time.Now(),
		},
	}

	for _, query := range defaultQueries {
		// Check if query already exists
		existingQuery := bson.M{
			"query_pattern": query.QueryPattern,
			"provider":      query.Provider,
		}
		existing, err := c.DB.FindOne(existingQuery, c.GetCollectionName())
		if err != nil || existing == nil {
			// Query doesn't exist, create it
			_, err := c.CreateQuery(&query)
			if err != nil {
				log.Printf("Error seeding default query for %s: %v", query.Provider, err)
				continue
			}
			log.Printf("Seeded default query for %s", query.Provider)
		}
	}

	return nil
}
