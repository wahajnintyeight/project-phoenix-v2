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
)

// RepoInfo contains repository information for discovered keys
type RepoInfo struct {
	RepoURL   string
	RepoOwner string
	RepoName  string
	FileURL   string
	FilePath  string
}

// ScraperHandler handles GitHub and GitLab scraping operations
type ScraperHandler struct {
	githubClient         *GitHubClient
	gitlabClient         *GitLabClient
	apiKeyController     *controllers.APIKeyController
	configController     *controllers.ScraperConfigController
	githubSearchLimiter  *RateLimiter // Separate limiter for GitHub Search API (30 req/min)
	githubContentLimiter *RateLimiter // Separate limiter for GitHub Content API (5000 req/hr)
	gitlabRateLimiter    *RateLimiter
	broker               broker.Broker
	processedCount       int
	duplicateCount       int
	errorCount           int
	scrapingCycles       int
	lastScrape           time.Time
	mutex                sync.Mutex
	gitlabEnabled        bool
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
	// GitHub Search API: 30 requests per minute, burst of 5
	// This allows 5 concurrent search requests, then throttles to 30/min average
	githubSearchLimiter := NewRateLimiter(30, 5)

	// GitHub Content API: 5000 requests per hour = ~83 requests per minute
	// Burst of 20 allows multiple concurrent file fetches
	githubContentLimiter := NewRateLimiter(83, 20)

	// GitLab: 10 requests per second = 600 per minute, burst of 10
	gitlabRateLimiter := NewRateLimiter(600, 10)

	// Get GitHub tokens from environment (already validated by config)
	githubTokens := strings.Split(os.Getenv("GITHUB_API_TOKEN"), ",")

	// Initialize GitHub client with both rate limiters
	githubClient := NewGitHubClient(githubTokens, githubSearchLimiter, githubContentLimiter)

	// Check if GitLab is enabled and initialize client
	var gitlabClient *GitLabClient
	gitlabEnabled := false
	gitlabTokensEnv := os.Getenv("GITLAB_API_TOKEN")
	if gitlabTokensEnv != "" {
		gitlabTokens := strings.Split(gitlabTokensEnv, ",")
		gitlabClient = NewGitLabClient(gitlabTokens, gitlabRateLimiter)
		gitlabEnabled = true
		log.Println("GitLab search enabled")
	} else {
		log.Println("GitLab search disabled (no GITLAB_API_TOKEN found)")
	}

	// Get controllers
	apiKeyController := controllers.GetControllerInstance(enum.APIKeyController, enum.MONGODB).(*controllers.APIKeyController)
	configController := controllers.GetControllerInstance(enum.ScraperConfigController, enum.MONGODB).(*controllers.ScraperConfigController)

	return &ScraperHandler{
		githubClient:         githubClient,
		gitlabClient:         gitlabClient,
		apiKeyController:     apiKeyController,
		configController:     configController,
		githubSearchLimiter:  githubSearchLimiter,
		githubContentLimiter: githubContentLimiter,
		gitlabRateLimiter:    gitlabRateLimiter,
		broker:               brokerObj,
		processedCount:       0,
		duplicateCount:       0,
		errorCount:           0,
		scrapingCycles:       0,
		lastScrape:           time.Time{},
		gitlabEnabled:        gitlabEnabled,
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

	// Check if we should randomly select a subset of queries
	maxQueriesPerCycle := h.getMaxQueriesPerCycle()
	selectedQueries := h.selectQueries(queries, maxQueriesPerCycle)

	if len(selectedQueries) < len(queries) {
		helper.LogInfo(ctx, "Randomly selected %d out of %d queries for this cycle", len(selectedQueries), len(queries))
	}

	// Process queries sequentially to respect GitHub rate limits
	// Processing concurrently causes all queries to queue up and hit rate limits quickly
	for _, query := range selectedQueries {
		if err := h.processQuery(query, correlationID); err != nil {
			queryCtx := helper.LogContext{
				ServiceName:   "scraper-service",
				Operation:     "processQuery",
				CorrelationID: correlationID,
			}
			helper.LogError(queryCtx, "Error processing query %s", err, query.QueryPattern)
			h.incrementError()
		}

		// No need for artificial delays - rate limiter handles this automatically
	}

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

	totalKeysFound := 0

	// Search GitHub using exhaustive search (bypasses 1000-result cap)
	helper.LogInfo(ctx, "Executing GitHub Code Search API request with size-based bisection")
	githubResults, err := h.githubClient.SearchCodeAll(query.QueryPattern, correlationID)
	if err != nil {
		helper.LogError(ctx, fmt.Sprintf("GitHub search failed - Query Pattern: %s, Provider: %s", query.QueryPattern, query.Provider), err)
	} else {
		helper.LogInfo(ctx, "Found %d GitHub results for query: %s", len(githubResults), query.QueryPattern)
		keysFound := h.processGitHubResultsConcurrently(githubResults, query.Provider, correlationID)
		totalKeysFound += keysFound
	}

	// Search GitLab if enabled
	if h.gitlabEnabled && h.gitlabClient != nil {
		helper.LogInfo(ctx, "Executing GitLab Code Search API request")
		gitlabResults, err := h.gitlabClient.SearchCode(query.QueryPattern, correlationID)
		if err != nil {
			helper.LogError(ctx, "GitLab search failed", err)
		} else {
			helper.LogInfo(ctx, "Found %d GitLab results for query: %s", len(gitlabResults), query.QueryPattern)
			keysFound := h.processGitLabResultsConcurrently(gitlabResults, query.Provider, correlationID)
			totalKeysFound += keysFound
		}
	}

	// Update query statistics
	helper.LogInfo(ctx, "Updating query statistics in MongoDB (total keys: %d)", totalKeysFound)
	if err := h.configController.UpdateQueryStats(query.ID, totalKeysFound); err != nil {
		helper.LogError(ctx, "Failed to update query stats in MongoDB", err)
	}

	return nil
}

// processGitHubResultsConcurrently processes GitHub search results using a worker pool
// EXPLANATION: Instead of processing results one-by-one, we create multiple workers
// that can process results in parallel. This is like having multiple cashiers
// at a store instead of just one.
func (h *ScraperHandler) processGitHubResultsConcurrently(results []*github.CodeResult, provider string, correlationID string) int {
	if len(results) == 0 {
		return 0
	}

	// CONCEPT: Channels are Go's way of communicating between goroutines
	// Think of them as pipes where you can send and receive data

	// resultsChan: sends work items (search results) to workers
	resultsChan := make(chan *github.CodeResult, len(results))

	// keysChan: workers send back the number of keys they found
	keysChan := make(chan int, len(results))

	// CONCEPT: WaitGroup tracks how many goroutines are still running
	// It's like a counter that waits for all workers to finish
	var wg sync.WaitGroup

	// WORKER POOL: Create concurrent workers
	// Each worker is a goroutine that processes results from the channel
	// Default to 10 workers, but allow configuration via environment
	numWorkers := 10
	if envWorkers := os.Getenv("SCRAPER_WORKER_POOL_SIZE"); envWorkers != "" {
		if parsed, err := fmt.Sscanf(envWorkers, "%d", &numWorkers); err == nil && parsed == 1 {
			if numWorkers < 1 {
				numWorkers = 1
			} else if numWorkers > 50 {
				numWorkers = 50 // Cap at 50 to avoid overwhelming GitHub API
			}
		}
	}
	if len(results) < numWorkers {
		numWorkers = len(results) // Don't create more workers than results
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1) // Tell WaitGroup we're starting a new goroutine

		// CONCEPT: "go func()" creates a new goroutine (lightweight thread)
		// This function runs concurrently with the main code
		go func(workerID int) {
			defer wg.Done() // When this function exits, tell WaitGroup we're done

			// CONCEPT: "range" on a channel keeps reading until the channel is closed
			// Each worker continuously pulls results from the channel and processes them
			for result := range resultsChan {
				if err := h.processSearchResult(result, provider, correlationID); err != nil {
					ctx := helper.LogContext{
						ServiceName:   "scraper-service",
						Operation:     "processResultsConcurrently",
						CorrelationID: correlationID,
					}
					helper.LogError(ctx, "Error processing search result", err)
					h.incrementError()
					keysChan <- 0 // Send 0 keys found
					continue
				}
				keysChan <- 1 // Send 1 key found
			}
		}(i)
	}

	// Send all results to the workers through the channel
	// CONCEPT: This is like putting work items on a conveyor belt
	// Workers will pick them up and process them
	for _, result := range results {
		resultsChan <- result
	}
	close(resultsChan) // Close channel to signal no more work is coming

	// Wait for all workers to finish
	wg.Wait()
	close(keysChan) // Close the results channel

	// Count total keys found
	keysFound := 0
	for count := range keysChan {
		keysFound += count
	}

	return keysFound
}

// processSearchResult processes a single search result
// OPTIMIZATION: This now runs concurrently in multiple goroutines
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
	rawPath := result.GetPath()

	// Clean the file path to remove '..' and other path traversal elements
	// GitHub API rejects paths with '..' for security reasons
	cleanPath := cleanFilePath(rawPath)
	if cleanPath == "" {
		return fmt.Errorf("invalid file path after cleaning: %s", rawPath)
	}

	repoInfo := &RepoInfo{
		RepoURL:   result.Repository.GetHTMLURL(),
		RepoOwner: result.Repository.Owner.GetLogin(),
		RepoName:  result.Repository.GetName(),
		FileURL:   result.GetHTMLURL(),
		FilePath:  cleanPath,
	}

	// Get file content
	// EXPLANATION: Multiple goroutines can fetch different files simultaneously
	// This is much faster than waiting for each file one-by-one
	helper.LogInfo(ctx, "Fetching file content from GitHub: %s (original: %s)", repoInfo.FilePath, rawPath)
	content, err := h.githubClient.GetFileContent(repoInfo.RepoOwner, repoInfo.RepoName, repoInfo.FilePath, correlationID)
	if err != nil {
		helper.LogError(ctx, fmt.Sprintf("Failed to get file content from GitHub - Repo URL: %s, File Path: %s", repoInfo.RepoURL, repoInfo.FilePath), err)
		return fmt.Errorf("failed to get file content: %w", err)
	}

	// Extract keys from content
	keys := h.ExtractKeys(content, provider)
	if len(keys) == 0 {
		return nil
	}

	helper.LogInfo(ctx, "Extracted %d keys from %s", len(keys), repoInfo.FilePath)

	// OPTIMIZATION: Store keys concurrently
	// If a file has multiple keys, we can store them in parallel
	var wg sync.WaitGroup
	for _, keyValue := range keys {
		wg.Add(1)
		go func(kv string) {
			defer wg.Done()
			if err := h.StoreDiscoveredKey(kv, provider, repoInfo, correlationID); err != nil {
				helper.LogError(ctx, "Failed to store key", err)
				h.incrementError()
			}
		}(keyValue)
	}
	wg.Wait() // Wait for all keys to be stored

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

	// Upsert the key (create or update last_seen_at)
	helper.LogInfo(ctx, "Upserting API key in MongoDB (provider: %s)", provider)
	keyID, isNew, err := h.apiKeyController.UpsertByKeyValue(apiKey)
	if err != nil {
		helper.LogError(ctx, "Failed to upsert key in MongoDB", err)
		return fmt.Errorf("failed to upsert key: %w", err)
	}

	if isNew {
		helper.LogInfo(ctx, "New key discovered: %s (provider: %s)", maskKey(keyValue), provider)
	} else {
		helper.LogInfo(ctx, "Duplicate key detected, updated last_seen_at: %s", maskKey(keyValue))
		h.incrementDuplicate()
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

	searchRemaining, searchResetTime := h.githubSearchLimiter.GetQuotaInfo()
	contentRemaining, contentResetTime := h.githubContentLimiter.GetQuotaInfo()

	return map[string]interface{}{
		"scraping_cycles":  h.scrapingCycles,
		"keys_discovered":  h.processedCount,
		"duplicates_found": h.duplicateCount,
		"errors":           h.errorCount,
		"github_rate_limit": map[string]interface{}{
			"search": map[string]interface{}{
				"remaining": searchRemaining,
				"reset_at":  searchResetTime,
			},
			"content": map[string]interface{}{
				"remaining": contentRemaining,
				"reset_at":  contentResetTime,
			},
		},
		"last_scrape": h.lastScrape,
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

// getMaxQueriesPerCycle returns the maximum number of queries to process per cycle
// Can be configured via MAX_QUERIES_PER_CYCLE environment variable
// Default is 0 (process all queries)
func (h *ScraperHandler) getMaxQueriesPerCycle() int {
	maxQueries := os.Getenv("MAX_QUERIES_PER_CYCLE")
	if maxQueries == "" {
		return 0 // Process all queries by default
	}

	var count int
	if _, err := fmt.Sscanf(maxQueries, "%d", &count); err != nil {
		log.Printf("Invalid MAX_QUERIES_PER_CYCLE value, processing all queries: %v", err)
		return 0
	}

	if count < 0 {
		return 0
	}

	return count
}

// selectQueries randomly selects a subset of queries for this cycle
// If maxQueries is 0 or >= len(queries), returns all queries
func (h *ScraperHandler) selectQueries(queries []*model.SearchQuery, maxQueries int) []*model.SearchQuery {
	if maxQueries == 0 || maxQueries >= len(queries) {
		return queries
	}

	// Create a copy to avoid modifying the original slice
	selected := make([]*model.SearchQuery, len(queries))
	copy(selected, queries)

	// Shuffle using Fisher-Yates algorithm
	for i := len(selected) - 1; i > 0; i-- {
		j := time.Now().UnixNano() % int64(i+1)
		selected[i], selected[j] = selected[j], selected[i]
	}

	// Return the first maxQueries items
	return selected[:maxQueries]
}

// maskKey masks an API key for logging (shows first and last 4 characters)
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// cleanFilePath normalizes a file path by removing '..' and other path traversal elements
// This is necessary because GitHub API rejects paths with '..' for security reasons
func cleanFilePath(path string) string {
	// Split path into components
	parts := strings.Split(path, "/")

	// Build clean path by resolving '..' references
	var cleanParts []string
	for _, part := range parts {
		if part == "" || part == "." {
			// Skip empty parts and current directory references
			continue
		}
		if part == ".." {
			// Go up one directory (remove last part if exists)
			if len(cleanParts) > 0 {
				cleanParts = cleanParts[:len(cleanParts)-1]
			}
			continue
		}
		// Add normal path component
		cleanParts = append(cleanParts, part)
	}

	// Rejoin the path
	return strings.Join(cleanParts, "/")
}

// processGitLabResultsConcurrently processes GitLab search results using a worker pool
func (h *ScraperHandler) processGitLabResultsConcurrently(results []*GitLabBlobResult, provider string, correlationID string) int {
	if len(results) == 0 {
		return 0
	}

	resultsChan := make(chan *GitLabBlobResult, len(results))
	keysChan := make(chan int, len(results))
	var wg sync.WaitGroup

	// Create worker pool
	numWorkers := 10
	if envWorkers := os.Getenv("SCRAPER_WORKER_POOL_SIZE"); envWorkers != "" {
		if parsed, err := fmt.Sscanf(envWorkers, "%d", &numWorkers); err == nil && parsed == 1 {
			if numWorkers < 1 {
				numWorkers = 1
			} else if numWorkers > 50 {
				numWorkers = 50
			}
		}
	}
	if len(results) < numWorkers {
		numWorkers = len(results)
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for result := range resultsChan {
				if err := h.processGitLabSearchResult(result, provider, correlationID); err != nil {
					ctx := helper.LogContext{
						ServiceName:   "scraper-service",
						Operation:     "processGitLabResultsConcurrently",
						CorrelationID: correlationID,
					}
					helper.LogError(ctx, "Error processing GitLab search result", err)
					h.incrementError()
					keysChan <- 0
					continue
				}
				keysChan <- 1
			}
		}(i)
	}

	// Send all results to workers
	for _, result := range results {
		resultsChan <- result
	}
	close(resultsChan)

	// Wait for all workers to finish
	wg.Wait()
	close(keysChan)

	// Count total keys found
	keysFound := 0
	for count := range keysChan {
		keysFound += count
	}

	return keysFound
}

// processGitLabSearchResult processes a single GitLab search result
func (h *ScraperHandler) processGitLabSearchResult(result *GitLabBlobResult, provider string, correlationID string) error {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "processGitLabSearchResult",
		CorrelationID: correlationID,
	}

	// Get project details
	project, err := h.gitlabClient.GetProject(result.ProjectID, correlationID)
	if err != nil {
		helper.LogError(ctx, "Failed to get GitLab project details", err)
		return fmt.Errorf("failed to get project: %w", err)
	}

	// Extract keys from the code snippet
	keys := h.ExtractKeys(result.Data, provider)
	if len(keys) == 0 {
		helper.LogInfo(ctx, "No keys found in GitLab result: %s", result.Path)
		return nil
	}

	helper.LogInfo(ctx, "Found %d keys in GitLab file: %s/%s", len(keys), project.PathWithNamespace, result.Path)

	// Construct file URL
	branchOrRef := result.Ref
	if branchOrRef == "" {
		branchOrRef = project.DefaultBranch
	}
	fileURL := fmt.Sprintf("%s/-/blob/%s/%s", project.WebURL, branchOrRef, result.Path)

	// Store each discovered key
	repoInfo := &RepoInfo{
		RepoURL:   project.WebURL,
		RepoOwner: project.Namespace.FullPath,
		RepoName:  project.Name,
		FileURL:   fileURL,
		FilePath:  result.Path,
	}

	for _, keyValue := range keys {
		if err := h.StoreDiscoveredKey(keyValue, provider, repoInfo, correlationID); err != nil {
			helper.LogError(ctx, "Failed to store discovered key", err)
			continue
		}
	}

	return nil
}
