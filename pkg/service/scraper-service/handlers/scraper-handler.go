package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/pkg/helper"

	"github.com/google/go-github/v60/github"
	"go-micro.dev/v4/broker"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// RepoInfo contains repository information for discovered keys
type RepoInfo struct {
	RepoURL   string
	RepoOwner string
	RepoName  string
	FileURL   string
	FilePath  string
}

// ScraperHandler handles GitHub scraping operations
type ScraperHandler struct {
	githubClient     *GitHubClient
	apiKeyController *controllers.APIKeyController
	configController *controllers.ScraperConfigController
	rateLimiter      *RateLimiter
	broker           broker.Broker
	processedCount   int
	duplicateCount   int
	errorCount       int
	scrapingCycles   int
	lastScrape       time.Time
	mutex            sync.Mutex
}

// Key extraction patterns for different providers
var KeyPatterns = map[string]*regexp.Regexp{
	model.ProviderOpenAI:     regexp.MustCompile(`sk-[a-zA-Z0-9]{48}`),
	model.ProviderAnthropic:  regexp.MustCompile(`sk-ant-[a-zA-Z0-9\-]{95,}`),
	model.ProviderGoogle:     regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`),
	model.ProviderOpenRouter: regexp.MustCompile(`sk-or-v1-[a-zA-Z0-9]{64}`),
}

// NewScraperHandler creates a new scraper handler
func NewScraperHandler(brokerObj broker.Broker) *ScraperHandler {
	// Initialize rate limiter with 5-second minimum delay
	rateLimiter := NewRateLimiter(5 * time.Second)

	// Get GitHub tokens from environment
	githubTokens := strings.Split(os.Getenv("GITHUB_API_TOKEN"), ",")
	if len(githubTokens) == 0 || githubTokens[0] == "" {
		log.Fatal("GITHUB_API_TOKEN must be set in environment")
	}

	// Initialize GitHub client
	githubClient := NewGitHubClient(githubTokens, rateLimiter)

	// Get controllers
	apiKeyController := controllers.GetControllerInstance(enum.APIKeyController, enum.MONGODB).(*controllers.APIKeyController)
	configController := controllers.GetControllerInstance(enum.ScraperConfigController, enum.MONGODB).(*controllers.ScraperConfigController)

	return &ScraperHandler{
		githubClient:     githubClient,
		apiKeyController: apiKeyController,
		configController: configController,
		rateLimiter:      rateLimiter,
		broker:           brokerObj,
		processedCount:   0,
		duplicateCount:   0,
		errorCount:       0,
		scrapingCycles:   0,
		lastScrape:       time.Time{},
	}
}

// RunScrapingCycle executes a complete scraping cycle
func (h *ScraperHandler) RunScrapingCycle() error {
	correlationID := helper.GenerateCorrelationID()
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "RunScrapingCycle",
		CorrelationID: correlationID,
	}

	h.mutex.Lock()
	h.scrapingCycles++
	h.lastScrape = time.Now()
	h.mutex.Unlock()

	helper.LogInfo(ctx, "Starting scraping cycle")

	// Get enabled queries
	helper.LogInfo(ctx, "Retrieving enabled queries from MongoDB")
	queries, err := h.configController.GetEnabledQueries()
	if err != nil {
		h.incrementError()
		helper.LogError(ctx, "Failed to get enabled queries from MongoDB", err)
		return fmt.Errorf("failed to get enabled queries: %w", err)
	}

	if len(queries) == 0 {
		helper.LogInfo(ctx, "No enabled queries found")
		return nil
	}

	helper.LogInfo(ctx, "Found %d enabled queries", len(queries))

	// Process queries concurrently
	var wg sync.WaitGroup
	for _, query := range queries {
		wg.Add(1)
		go func(q *model.SearchQuery) {
			defer wg.Done()
			if err := h.processQuery(q, correlationID); err != nil {
				queryCtx := helper.LogContext{
					ServiceName:   "scraper-service",
					Operation:     "processQuery",
					CorrelationID: correlationID,
				}
				helper.LogError(queryCtx, "Error processing query %s", err, q.QueryPattern)
				h.incrementError()
			}
		}(query)
	}

	wg.Wait()

	helper.LogInfo(ctx, "Scraping cycle completed")
	return nil
}

// processQuery processes a single search query
func (h *ScraperHandler) processQuery(query *model.SearchQuery, correlationID string) error {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "processQuery",
		CorrelationID: correlationID,
	}

	helper.LogInfo(ctx, "Processing query: %s (provider: %s)", query.QueryPattern, query.Provider)

	// Search GitHub
	helper.LogInfo(ctx, "Executing GitHub Code Search API request")
	results, err := h.githubClient.SearchCode(query.QueryPattern, correlationID)
	if err != nil {
		helper.LogError(ctx, "GitHub search failed", err)
		return fmt.Errorf("GitHub search failed: %w", err)
	}

	helper.LogInfo(ctx, "Found %d results for query: %s", len(results), query.QueryPattern)

	// Process results
	keysFound := 0
	for _, result := range results {
		if err := h.processSearchResult(result, query.Provider, correlationID); err != nil {
			helper.LogError(ctx, "Error processing search result", err)
			h.incrementError()
			continue
		}
		keysFound++
	}

	// Update query statistics
	helper.LogInfo(ctx, "Updating query statistics in MongoDB")
	if err := h.configController.UpdateQueryStats(query.ID, keysFound); err != nil {
		helper.LogError(ctx, "Failed to update query stats in MongoDB", err)
	}

	return nil
}

// processSearchResult processes a single search result
func (h *ScraperHandler) processSearchResult(result *github.CodeResult, provider string, correlationID string) error {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "processSearchResult",
		CorrelationID: correlationID,
	}

	if result.Repository == nil || result.Repository.Owner == nil {
		return fmt.Errorf("invalid search result: missing repository information")
	}

	// Extract repository information
	repoInfo := &RepoInfo{
		RepoURL:   result.Repository.GetHTMLURL(),
		RepoOwner: result.Repository.Owner.GetLogin(),
		RepoName:  result.Repository.GetName(),
		FileURL:   result.GetHTMLURL(),
		FilePath:  result.GetPath(),
	}

	// Get file content
	helper.LogInfo(ctx, "Fetching file content from GitHub: %s", repoInfo.FilePath)
	content, err := h.githubClient.GetFileContent(repoInfo.RepoOwner, repoInfo.RepoName, repoInfo.FilePath, correlationID)
	if err != nil {
		helper.LogError(ctx, "Failed to get file content from GitHub", err)
		return fmt.Errorf("failed to get file content: %w", err)
	}

	// Extract keys from content
	keys := h.ExtractKeys(content, provider)
	if len(keys) == 0 {
		return nil
	}

	helper.LogInfo(ctx, "Extracted %d keys from %s", len(keys), repoInfo.FilePath)

	// Store discovered keys
	for _, keyValue := range keys {
		if err := h.StoreDiscoveredKey(keyValue, provider, repoInfo, correlationID); err != nil {
			helper.LogError(ctx, "Failed to store key", err)
			h.incrementError()
		}
	}

	return nil
}

// ExtractKeys extracts API keys from content using provider-specific patterns
func (h *ScraperHandler) ExtractKeys(content string, provider string) []string {
	pattern, ok := KeyPatterns[provider]
	if !ok {
		log.Printf("No pattern found for provider: %s", provider)
		return nil
	}

	matches := pattern.FindAllString(content, -1)

	// Deduplicate matches
	uniqueKeys := make(map[string]bool)
	for _, match := range matches {
		uniqueKeys[match] = true
	}

	keys := make([]string, 0, len(uniqueKeys))
	for key := range uniqueKeys {
		keys = append(keys, key)
	}

	return keys
}

// StoreDiscoveredKey stores a discovered key with deduplication logic
func (h *ScraperHandler) StoreDiscoveredKey(keyValue string, provider string, repoInfo *RepoInfo, correlationID string) error {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "StoreDiscoveredKey",
		CorrelationID: correlationID,
	}

	h.incrementProcessed()

	// Create API key model
	apiKey := &model.APIKey{
		KeyValue:   keyValue,
		Provider:   provider,
		Status:     model.StatusPending,
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
		ErrorCount: 0,
		RepoRefs:   []primitive.ObjectID{},
	}

	// Try to create the key
	helper.LogInfo(ctx, "Storing API key in MongoDB (provider: %s)", provider)
	keyID, err := h.apiKeyController.Create(apiKey)
	if err != nil {
		// Check if it's a duplicate key error
		if mongo.IsDuplicateKeyError(err) {
			helper.LogInfo(ctx, "Duplicate key detected: %s", maskKey(keyValue))
			h.incrementDuplicate()

			// Update LastSeenAt timestamp
			helper.LogInfo(ctx, "Updating last_seen_at timestamp in MongoDB")
			if err := h.apiKeyController.UpdateLastSeen(keyValue); err != nil {
				helper.LogError(ctx, "Failed to update last seen in MongoDB", err)
				return fmt.Errorf("failed to update last seen: %w", err)
			}

			// Get existing key to add repo reference
			existingKey, err := h.apiKeyController.FindByKeyValue(keyValue)
			if err != nil {
				helper.LogError(ctx, "Failed to find existing key in MongoDB", err)
				return fmt.Errorf("failed to find existing key: %w", err)
			}
			keyID = existingKey.ID
		} else {
			helper.LogError(ctx, "Failed to create key in MongoDB", err)
			return fmt.Errorf("failed to create key: %w", err)
		}
	} else {
		helper.LogInfo(ctx, "New key discovered: %s (provider: %s)", maskKey(keyValue), provider)
	}

	// Add repository reference
	repoRef := &model.RepoReference{
		APIKeyID:  keyID,
		RepoURL:   repoInfo.RepoURL,
		RepoOwner: repoInfo.RepoOwner,
		RepoName:  repoInfo.RepoName,
		FileURL:   repoInfo.FileURL,
		FilePath:  repoInfo.FilePath,
		FoundAt:   time.Now(),
	}

	helper.LogInfo(ctx, "Adding repository reference to MongoDB")
	if err := h.apiKeyController.AddRepoReference(keyID, repoRef); err != nil {
		helper.LogError(ctx, "Failed to add repo reference to MongoDB", err)
	}

	// Publish key discovered event
	apiKey.ID = keyID
	if err := h.PublishKeyDiscovered(apiKey, repoInfo, correlationID); err != nil {
		helper.LogError(ctx, "Failed to publish key discovered event to RabbitMQ", err)
	}

	return nil
}

// PublishKeyDiscovered publishes a key discovered event to RabbitMQ
func (h *ScraperHandler) PublishKeyDiscovered(key *model.APIKey, repoInfo *RepoInfo, correlationID string) error {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "PublishKeyDiscovered",
		CorrelationID: correlationID,
	}

	if h.broker == nil {
		helper.LogError(ctx, "RabbitMQ broker is unavailable, skipping publish", nil)
		return nil // Continue operation even if broker is unavailable
	}

	payload := map[string]interface{}{
		"id":            key.ID.Hex(),
		"key_value":     key.KeyValue,
		"provider":      key.Provider,
		"repo_url":      repoInfo.RepoURL,
		"file_path":     repoInfo.FilePath,
		"discovered_at": key.CreatedAt,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		helper.LogError(ctx, "Failed to marshal RabbitMQ payload", err)
		return nil // Continue operation even if marshal fails
	}

	msg := &broker.Message{
		Body: data,
	}

	helper.LogInfo(ctx, "Publishing message to RabbitMQ topic: keys.discovered")
	if err := h.broker.Publish("keys.discovered", msg); err != nil {
		helper.LogError(ctx, "Failed to publish message to RabbitMQ, continuing operation", err)
		return nil // Log error but continue operation
	}

	helper.LogInfo(ctx, "Published key discovered event for key: %s", key.ID.Hex())
	return nil
}

// GetStats returns handler statistics
func (h *ScraperHandler) GetStats() map[string]interface{} {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	remaining, resetTime := h.rateLimiter.GetQuotaInfo()

	return map[string]interface{}{
		"scraping_cycles":      h.scrapingCycles,
		"keys_discovered":      h.processedCount,
		"duplicates_found":     h.duplicateCount,
		"errors":               h.errorCount,
		"rate_limit_remaining": remaining,
		"rate_limit_reset":     resetTime,
		"last_scrape":          h.lastScrape,
	}
}

// Helper methods for thread-safe counter updates
func (h *ScraperHandler) incrementProcessed() {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.processedCount++
}

func (h *ScraperHandler) incrementDuplicate() {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.duplicateCount++
}

func (h *ScraperHandler) incrementError() {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.errorCount++
}

// maskKey masks an API key for logging (shows first and last 4 characters)
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
