package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"project-phoenix/v2/pkg/helper"
	"strconv"
	"time"

	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

// GitHubClient wraps the GitHub API client with rate limiting and retry logic
type GitHubClient struct {
	client         *github.Client
	tokens         []string
	currentToken   int
	searchLimiter  *RateLimiter // For Search API (30 req/min)
	contentLimiter *RateLimiter // For Content API (5000 req/hr)
	requestTimeout time.Duration
	ctx            context.Context
}

// NewGitHubClient creates a new GitHub client with token rotation support
func NewGitHubClient(tokens []string, searchLimiter *RateLimiter, contentLimiter *RateLimiter) *GitHubClient {
	// Token validation is done by config, this should never be empty

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: tokens[0]},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &GitHubClient{
		client:         github.NewClient(tc),
		tokens:         tokens,
		currentToken:   0,
		searchLimiter:  searchLimiter,
		contentLimiter: contentLimiter,
		requestTimeout: 30 * time.Second,
		ctx:            ctx,
	}
}

// SearchCode searches GitHub for code matching the query with pagination support
// This method is capped at 1000 results due to GitHub's API limitation
// SearchCode searches GitHub for code matching the query with pagination support
// This method is capped at 1000 results due to GitHub's API limitation
// Returns results and total count available from GitHub
func (c *GitHubClient) SearchCode(query string, correlationID string) ([]*github.CodeResult, int, error) {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitHubClient.SearchCode",
		CorrelationID: correlationID,
	}

	log.Printf("Executing GitHub Code Search with query: %s", query)

	var allResults []*github.CodeResult
	var totalAvailable int
	page := 1
	maxPages := 10 // GitHub allows max 1000 results (10 pages * 100 per page)

	for page <= maxPages {
		// Wait for rate limiter before each request (Search API limiter)
		c.searchLimiter.Wait()

		// Check if we should pause due to low quota
		if c.searchLimiter.ShouldPause() {
			remaining, resetTime := c.searchLimiter.GetQuotaInfo()
			waitDuration := time.Until(resetTime)
			helper.LogInfo(ctx, "Search API rate limit quota low (%d remaining), pausing until %v (waiting %v)", remaining, resetTime, waitDuration)
			if waitDuration > 0 {
				time.Sleep(waitDuration)
			}
		}

		// Create context with timeout for this page
		reqCtx, cancel := context.WithTimeout(c.ctx, c.requestTimeout)

		// Perform search with retry logic
		var result *github.CodeSearchResult
		var resp *github.Response
		var err error

		log.Printf("GitHub API request: page=%d, per_page=100, query=%s", page, query)

		for attempt := 0; attempt < 3; attempt++ {
			result, resp, err = c.client.Search.Code(reqCtx, query, &github.SearchOptions{
				ListOptions: github.ListOptions{
					Page:    page,
					PerPage: 100, // GitHub max is 100
				},
			})

			if err == nil {
				// Update rate limit from response headers
				if resp != nil && resp.Rate.Remaining > 0 {
					resetTime := resp.Rate.Reset.Time
					c.searchLimiter.UpdateQuota(resp.Rate.Remaining, resetTime)
					log.Printf("GitHub Search API rate limit: %d remaining, resets at %v", resp.Rate.Remaining, resetTime)
				}
				break
			}

			// Handle rate limit errors
			if resp != nil && resp.StatusCode == 429 {
				helper.LogInfo(ctx, "GitHub API rate limit exceeded (HTTP 429)")
				if err := c.WaitForRateLimit(resp, correlationID); err != nil {
					cancel()
					helper.LogError(ctx, "Rate limit wait failed", err)
					return allResults, totalAvailable, fmt.Errorf("rate limit wait failed: %w", err)
				}
				continue
			}

			// Retry on 5xx errors with exponential backoff
			if resp != nil && resp.StatusCode >= 500 {
				backoff := time.Duration(1<<uint(attempt)) * time.Second
				helper.LogInfo(ctx, "GitHub API returned %d, retrying in %v (attempt %d/3)", resp.StatusCode, backoff, attempt+1)
				time.Sleep(backoff)
				continue
			}

			// Other errors, don't retry
			helper.LogError(ctx, fmt.Sprintf("GitHub search failed - Query: %s", query), err)
			cancel()
			return allResults, totalAvailable, fmt.Errorf("GitHub search failed: %w", err)
		}

		cancel() // Clean up context

		if err != nil {
			helper.LogError(ctx, fmt.Sprintf("GitHub search failed after retries on page %d - Query: %s", page, query), err)
			return allResults, totalAvailable, fmt.Errorf("GitHub search failed after retries: %w", err)
		}

		// Add results from this page
		pageResults := len(result.CodeResults)
		allResults = append(allResults, result.CodeResults...)
		totalAvailable = result.GetTotal()

		log.Printf("GitHub search page %d: found %d results (total so far: %d, total available: %d)",
			page, pageResults, len(allResults), totalAvailable)

		// Check if we got 0 results - could mean end or pagination issue
		if pageResults == 0 {
			// If we have results but got 0 on this page, and total available is higher, it's a pagination issue
			if len(allResults) > 0 && totalAvailable > len(allResults) {
				log.Printf("GitHub search: got 0 results on page %d but %d total available (pagination issue), stopping", page, totalAvailable)
			} else {
				log.Printf("GitHub search completed: reached last page at page %d", page)
			}
			break
		}

		// Check if we got fewer results than requested (last page)
		if pageResults < 100 {
			log.Printf("GitHub search completed: reached last page at page %d", page)
			break
		}

		// Check if we've reached GitHub's 1000 result limit
		if len(allResults) >= 1000 {
			log.Printf("GitHub search completed: reached GitHub's 1000 result limit")
			break
		}

		// Check if there are more results available
		if totalAvailable <= len(allResults) {
			log.Printf("GitHub search completed: fetched all %d available results", len(allResults))
			break
		}

		page++
	}

	log.Printf("GitHub search completed: found %d total results across %d pages", len(allResults), page)
	return allResults, totalAvailable, nil
}

// searchSizeRange recursively searches within a file size range, bisecting when hitting the 1000-result cap
func (c *GitHubClient) searchSizeRange(query string, minBytes, maxBytes int, maxResults int, currentCount int, correlationID string) ([]*github.CodeResult, error) {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitHubClient.searchSizeRange",
		CorrelationID: correlationID,
	}

	// Check if we've reached the max result cap
	if maxResults > 0 && currentCount >= maxResults {
		helper.LogInfo(ctx, "Reached MAX_RESULT_CAP of %d results, stopping search", maxResults)
		return nil, nil
	}

	// Build query with size range
	rangeQuery := fmt.Sprintf("%s size:%d..%d", query, minBytes, maxBytes)
	helper.LogInfo(ctx, "Searching size range: %d..%d bytes (current count: %d)", minBytes, maxBytes, currentCount)

	// Search with the size-constrained query
	results, totalAvailable, err := c.SearchCode(rangeQuery, correlationID)
	if err != nil {
		return nil, err
	}

	// Check if bisection is needed:
	// 1. Hit the 1000-result cap, OR
	// 2. Got fewer results than available (pagination issue)
	needsBisection := (len(results) >= 1000 || (totalAvailable > len(results) && len(results) > 0)) && minBytes < maxBytes

	// If no bisection needed, return results
	if !needsBisection {
		if totalAvailable > len(results) && len(results) > 0 {
			helper.LogInfo(ctx, "Size range %d..%d: found %d results but %d available (pagination issue detected, but can't bisect further)", minBytes, maxBytes, len(results), totalAvailable)
		} else {
			helper.LogInfo(ctx, "Size range %d..%d: found %d results (no bisection needed)", minBytes, maxBytes, len(results))
		}

		// Trim results if we would exceed max cap
		if maxResults > 0 {
			remaining := maxResults - currentCount
			if remaining <= 0 {
				return nil, nil
			}
			if len(results) > remaining {
				helper.LogInfo(ctx, "Trimming results from %d to %d to respect MAX_RESULT_CAP", len(results), remaining)
				results = results[:remaining]
			}
		}

		return results, nil
	}

	// Hit the 1000-result cap or pagination issue - bisect the size range
	mid := (minBytes + maxBytes) / 2
	if len(results) >= 1000 {
		helper.LogInfo(ctx, "Size range %d..%d hit 1000-result cap, bisecting at %d bytes", minBytes, maxBytes, mid)
	} else {
		helper.LogInfo(ctx, "Size range %d..%d has pagination issue (%d results but %d available), bisecting at %d bytes", minBytes, maxBytes, len(results), totalAvailable, mid)
	}

	// Search left half
	left, err := c.searchSizeRange(query, minBytes, mid, maxResults, currentCount, correlationID)
	if err != nil {
		return nil, fmt.Errorf("left bisection failed: %w", err)
	}

	// Update current count with left results
	newCount := currentCount + len(left)

	// Search right half (only if we haven't hit the cap)
	var right []*github.CodeResult
	if maxResults == 0 || newCount < maxResults {
		right, err = c.searchSizeRange(query, mid+1, maxBytes, maxResults, newCount, correlationID)
		if err != nil {
			return nil, fmt.Errorf("right bisection failed: %w", err)
		}
	} else {
		helper.LogInfo(ctx, "Skipping right bisection, already at MAX_RESULT_CAP")
	}

	// Combine results
	combined := append(left, right...)
	helper.LogInfo(ctx, "Size range %d..%d: combined %d results (left: %d, right: %d)", minBytes, maxBytes, len(combined), len(left), len(right))
	return combined, nil
}

// SearchCodeAll bypasses GitHub's 1000-result cap by recursively bisecting on file size
// Uses the size: qualifier which IS supported for code search (unlike created: or pushed:)
// Respects MAX_RESULT_CAP environment variable (0 = unlimited)
func (c *GitHubClient) SearchCodeAll(query string, correlationID string) ([]*github.CodeResult, error) {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitHubClient.SearchCodeAll",
		CorrelationID: correlationID,
	}

	// Get max result cap from environment (0 = unlimited)
	maxResults := c.getMaxResultCap()
	if maxResults > 0 {
		helper.LogInfo(ctx, "Starting search with MAX_RESULT_CAP=%d for query: %s", maxResults, query)
	} else {
		helper.LogInfo(ctx, "Starting exhaustive search (unlimited) with size-based bisection for query: %s", query)
	}

	// GitHub's max indexed file size is 384KB
	const maxFileSize = 384 * 1024
	allResults, err := c.searchSizeRange(query, 0, maxFileSize, maxResults, 0, correlationID)
	if err != nil {
		return nil, err
	}

	// Deduplicate by SHA (files with identical content share the same SHA)
	seen := make(map[string]struct{})
	var unique []*github.CodeResult
	duplicates := 0

	for _, r := range allResults {
		sha := r.GetSHA()
		if sha == "" {
			// Include results without SHA (shouldn't happen, but be safe)
			unique = append(unique, r)
			continue
		}

		if _, exists := seen[sha]; !exists {
			seen[sha] = struct{}{}
			unique = append(unique, r)
		} else {
			duplicates++
		}
	}

	if maxResults > 0 {
		helper.LogInfo(ctx, "Search completed with cap: %d total results, %d unique (removed %d duplicates)", len(allResults), len(unique), duplicates)
	} else {
		helper.LogInfo(ctx, "Exhaustive search completed: %d total results, %d unique (removed %d duplicates)", len(allResults), len(unique), duplicates)
	}
	return unique, nil
}

// getMaxResultCap returns the maximum number of results to fetch per query
// Configured via MAX_RESULT_CAP environment variable
// Returns 0 for unlimited (default)
func (c *GitHubClient) getMaxResultCap() int {
	maxCap := os.Getenv("MAX_RESULT_CAP")
	if maxCap == "" {
		return 0 // Unlimited by default
	}

	var count int
	if _, err := fmt.Sscanf(maxCap, "%d", &count); err != nil {
		log.Printf("Invalid MAX_RESULT_CAP value '%s', using unlimited: %v", maxCap, err)
		return 0
	}

	if count < 0 {
		return 0 // Treat negative as unlimited
	}

	return count
}

// GetFileContent retrieves the content of a file from GitHub
func (c *GitHubClient) GetFileContent(owner, repo, path string, correlationID string) (string, error) {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitHubClient.GetFileContent",
		CorrelationID: correlationID,
	}

	// Use Content API rate limiter (much higher limit than Search API)
	c.contentLimiter.Wait()

	reqCtx, cancel := context.WithTimeout(c.ctx, c.requestTimeout)
	defer cancel()

	helper.LogInfo(ctx, "Fetching file content from GitHub: %s/%s/%s", owner, repo, path)
	fileContent, _, resp, err := c.client.Repositories.GetContents(reqCtx, owner, repo, path, nil)
	if err != nil {
		helper.LogError(ctx, fmt.Sprintf("Failed to get file content from GitHub - Repo: %s/%s, Path: %s", owner, repo, path), err)
		return "", fmt.Errorf("failed to get file content: %w", err)
	}

	// Update rate limit for Content API
	if resp != nil && resp.Rate.Remaining > 0 {
		c.contentLimiter.UpdateQuota(resp.Rate.Remaining, resp.Rate.Reset.Time)
		// log.Printf("GitHub Content API rate limit: %d remaining", resp.Rate.Remaining)
	}

	if fileContent == nil {
		return "", fmt.Errorf("file content is nil")
	}

	content, err := fileContent.GetContent()
	if err != nil {
		helper.LogError(ctx, "Failed to decode file content", err)
		return "", fmt.Errorf("failed to decode file content: %w", err)
	}

	return content, nil
}

// CheckRateLimit queries the current rate limit status
func (c *GitHubClient) CheckRateLimit() (*github.RateLimits, error) {
	ctx, cancel := context.WithTimeout(c.ctx, c.requestTimeout)
	defer cancel()

	rateLimits, _, err := c.client.RateLimit.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check rate limit: %w", err)
	}

	// Update internal rate limiters
	if rateLimits != nil {
		if rateLimits.Search != nil {
			c.searchLimiter.UpdateQuota(rateLimits.Search.Remaining, rateLimits.Search.Reset.Time)
		}
		if rateLimits.Core != nil {
			c.contentLimiter.UpdateQuota(rateLimits.Core.Remaining, rateLimits.Core.Reset.Time)
		}
	}

	return rateLimits, nil
}

// WaitForRateLimit handles HTTP 429 responses using Retry-After header
func (c *GitHubClient) WaitForRateLimit(resp *github.Response, correlationID string) error {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitHubClient.WaitForRateLimit",
		CorrelationID: correlationID,
	}

	if resp == nil {
		return fmt.Errorf("response is nil")
	}

	// Check for Retry-After header
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter != "" {
		// Parse as seconds
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			waitDuration := time.Duration(seconds) * time.Second
			helper.LogInfo(ctx, "Rate limited, waiting %v as specified in Retry-After header", waitDuration)
			time.Sleep(waitDuration)
			return nil
		}

		// Parse as HTTP date
		if retryTime, err := time.Parse(time.RFC1123, retryAfter); err == nil {
			waitDuration := time.Until(retryTime)
			if waitDuration > 0 {
				helper.LogInfo(ctx, "Rate limited, waiting until %v (%v)", retryTime, waitDuration)
				time.Sleep(waitDuration)
				return nil
			}
		}
	}

	// Fallback: use rate limit reset time
	if resp.Rate.Reset.Time.After(time.Now()) {
		waitDuration := time.Until(resp.Rate.Reset.Time)
		helper.LogInfo(ctx, "Rate limited, waiting until reset at %v (%v)", resp.Rate.Reset.Time, waitDuration)
		time.Sleep(waitDuration)
		return nil
	}

	// Default wait
	helper.LogInfo(ctx, "Rate limited, waiting 60 seconds (default)")
	time.Sleep(60 * time.Second)
	return nil
}

// RotateToken switches to the next available GitHub token
func (c *GitHubClient) RotateToken() {
	if len(c.tokens) <= 1 {
		log.Println("Only one token available, cannot rotate")
		return
	}

	c.currentToken = (c.currentToken + 1) % len(c.tokens)
	log.Printf("Rotating to token %d/%d", c.currentToken+1, len(c.tokens))

	// Create new client with rotated token
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: c.tokens[c.currentToken]},
	)
	tc := oauth2.NewClient(c.ctx, ts)
	c.client = github.NewClient(tc)
}
