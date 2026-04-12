package handlers

import (
	"context"
	"fmt"
	"log"
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
func (c *GitHubClient) SearchCode(query string, correlationID string) ([]*github.CodeResult, error) {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitHubClient.SearchCode",
		CorrelationID: correlationID,
	}

	log.Printf("Executing GitHub Code Search with query: %s", query)

	var allResults []*github.CodeResult
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
				Sort:  "indexed", // Sort by recently indexed files
				Order: "desc",    // Most recent first
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
					return allResults, fmt.Errorf("rate limit wait failed: %w", err)
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
			return allResults, fmt.Errorf("GitHub search failed: %w", err)
		}

		cancel() // Clean up context

		if err != nil {
			helper.LogError(ctx, fmt.Sprintf("GitHub search failed after retries on page %d - Query: %s", page, query), err)
			return allResults, fmt.Errorf("GitHub search failed after retries: %w", err)
		}

		// Add results from this page
		pageResults := len(result.CodeResults)
		allResults = append(allResults, result.CodeResults...)

		log.Printf("GitHub search page %d: found %d results (total so far: %d, total available: %d)",
			page, pageResults, len(allResults), result.GetTotal())

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
		if result.GetTotal() <= len(allResults) {
			log.Printf("GitHub search completed: fetched all %d available results", len(allResults))
			break
		}

		page++
	}

	log.Printf("GitHub search completed: found %d total results across %d pages", len(allResults), page)
	return allResults, nil
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
