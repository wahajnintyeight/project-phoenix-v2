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
	rateLimiter    *RateLimiter
	requestTimeout time.Duration
	ctx            context.Context
}

// NewGitHubClient creates a new GitHub client with token rotation support
func NewGitHubClient(tokens []string, rateLimiter *RateLimiter) *GitHubClient {
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
		rateLimiter:    rateLimiter,
		requestTimeout: 30 * time.Second,
		ctx:            ctx,
	}
}

// SearchCode searches GitHub for code matching the query
func (c *GitHubClient) SearchCode(query string, correlationID string) ([]*github.CodeResult, error) {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitHubClient.SearchCode",
		CorrelationID: correlationID,
	}

	// Wait for rate limiter
	c.rateLimiter.Wait()

	// Check if we should pause due to low quota
	if c.rateLimiter.ShouldPause() {
		remaining, resetTime := c.rateLimiter.GetQuotaInfo()
		waitDuration := time.Until(resetTime)
		helper.LogInfo(ctx, "Rate limit quota low (%d remaining), pausing until %v (waiting %v)", remaining, resetTime, waitDuration)
		if waitDuration > 0 {
			time.Sleep(waitDuration)
		}
	}

	// Create context with timeout
	reqCtx, cancel := context.WithTimeout(c.ctx, c.requestTimeout)
	defer cancel()

	// Perform search with retry logic
	var result *github.CodeSearchResult
	var resp *github.Response
	var err error

	for attempt := 0; attempt < 3; attempt++ {
		result, resp, err = c.client.Search.Code(reqCtx, query, &github.SearchOptions{
			ListOptions: github.ListOptions{
				PerPage: 500,
			},
		})

		if err == nil {
			// Update rate limit from response headers
			if resp != nil && resp.Rate.Remaining > 0 {
				resetTime := resp.Rate.Reset.Time
				c.rateLimiter.UpdateQuota(resp.Rate.Remaining, resetTime)
				helper.LogInfo(ctx, "GitHub API rate limit: %d remaining, resets at %v", resp.Rate.Remaining, resetTime)
			}
			break
		}

		// Handle rate limit errors
		if resp != nil && resp.StatusCode == 429 {
			helper.LogInfo(ctx, "GitHub API rate limit exceeded (HTTP 429)")
			if err := c.WaitForRateLimit(resp, correlationID); err != nil {
				helper.LogError(ctx, "Rate limit wait failed", err)
				return nil, fmt.Errorf("rate limit wait failed: %w", err)
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
		helper.LogError(ctx, "GitHub search failed", err)
		return nil, fmt.Errorf("GitHub search failed: %w", err)
	}

	if err != nil {
		helper.LogError(ctx, "GitHub search failed after retries", err)
		return nil, fmt.Errorf("GitHub search failed after retries: %w", err)
	}

	helper.LogInfo(ctx, "GitHub search completed: found %d results", len(result.CodeResults))
	return result.CodeResults, nil
}

// GetFileContent retrieves the content of a file from GitHub
func (c *GitHubClient) GetFileContent(owner, repo, path string, correlationID string) (string, error) {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitHubClient.GetFileContent",
		CorrelationID: correlationID,
	}

	c.rateLimiter.Wait()

	reqCtx, cancel := context.WithTimeout(c.ctx, c.requestTimeout)
	defer cancel()

	helper.LogInfo(ctx, "Fetching file content from GitHub: %s/%s/%s", owner, repo, path)
	fileContent, _, resp, err := c.client.Repositories.GetContents(reqCtx, owner, repo, path, nil)
	if err != nil {
		helper.LogError(ctx, "Failed to get file content from GitHub", err)
		return "", fmt.Errorf("failed to get file content: %w", err)
	}

	// Update rate limit
	if resp != nil && resp.Rate.Remaining > 0 {
		c.rateLimiter.UpdateQuota(resp.Rate.Remaining, resp.Rate.Reset.Time)
		helper.LogInfo(ctx, "GitHub API rate limit: %d remaining", resp.Rate.Remaining)
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

	// Update internal rate limiter
	if rateLimits != nil && rateLimits.Core != nil {
		c.rateLimiter.UpdateQuota(rateLimits.Core.Remaining, rateLimits.Core.Reset.Time)
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
