package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"project-phoenix/v2/pkg/helper"
	"strconv"
	"time"
)

var ErrGitLabRepositoryUnavailable = errors.New("gitlab repository unavailable")

// GitLabBlobResult represents a single blob search result from GitLab
type GitLabBlobResult struct {
	ProjectID int    `json:"project_id"`
	Data      string `json:"data"`      // Code snippet containing the match
	Path      string `json:"path"`      // File path
	Filename  string `json:"filename"`  // Filename
	Ref       string `json:"ref"`       // Branch or commit SHA
	StartLine int    `json:"startline"` // Line number where match starts
}

// GitLabProject represents project details from GitLab
type GitLabProject struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Path              string `json:"path"`
	PathWithNamespace string `json:"path_with_namespace"`
	WebURL            string `json:"web_url"`
	DefaultBranch     string `json:"default_branch"`
	Description       string `json:"description"`
	Namespace         struct {
		FullPath string `json:"full_path"`
	} `json:"namespace"`
}

// GitLabTreeNode represents an item in a project's repository tree.
// For blob entries, ID is the blob SHA.
type GitLabTreeNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
	Mode string `json:"mode"`
}

// GitLabClient wraps the GitLab API client with rate limiting and retry logic
type GitLabClient struct {
	httpClient     *http.Client
	tokens         []string
	currentToken   int
	rateLimiter    *RateLimiter
	requestTimeout time.Duration
	baseURL        string
}

// NewGitLabClient creates a new GitLab client with token rotation support
func NewGitLabClient(tokens []string, rateLimiter *RateLimiter) *GitLabClient {
	return &GitLabClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		tokens:         tokens,
		currentToken:   0,
		rateLimiter:    rateLimiter,
		requestTimeout: 30 * time.Second,
		baseURL:        "https://gitlab.com",
	}
}

// ListPublicProjects fetches one page of public projects in stable ID order.
func (c *GitLabClient) ListPublicProjects(page, perPage int, correlationID string) ([]*GitLabProject, error) {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitLabClient.ListPublicProjects",
		CorrelationID: correlationID,
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 100
	}

	projectsURL := fmt.Sprintf("%s/api/v4/projects?visibility=public&order_by=id&sort=asc&page=%d&per_page=%d",
		c.baseURL, page, perPage)

	helper.LogInfo(ctx, "Scanning public GitLab projects (page=%d, per_page=%d)", page, perPage)

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		c.rateLimiter.Wait()

		req, err := http.NewRequest("GET", projectsURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("PRIVATE-TOKEN", c.tokens[c.currentToken])
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		c.updateRateLimitFromHeaders(resp.Header, correlationID)

		if resp.StatusCode == 429 {
			_ = c.WaitForRateLimit(resp, correlationID)
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limited")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(bodyBytes))
		}

		var projects []*GitLabProject
		if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode project list: %w", err)
		}
		resp.Body.Close()

		return projects, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to list public projects after retries: %w", lastErr)
	}

	return nil, fmt.Errorf("failed to list public projects after retries")
}

// GetRepositoryTree fetches full recursive repository tree for a project.
// GitLab paginates tree responses, so this method walks all pages.
func (c *GitLabClient) GetRepositoryTree(projectID int, correlationID string) ([]*GitLabTreeNode, error) {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitLabClient.GetRepositoryTree",
		CorrelationID: correlationID,
	}

	if projectID <= 0 {
		return nil, fmt.Errorf("invalid project ID: %d", projectID)
	}

	var allNodes []*GitLabTreeNode
	page := 1
	perPage := 100

	for {
		treeURL := fmt.Sprintf("%s/api/v4/projects/%d/repository/tree?recursive=true&page=%d&per_page=%d",
			c.baseURL, projectID, page, perPage)

		var resp *http.Response
		var lastErr error

		for attempt := 0; attempt < 3; attempt++ {
			c.rateLimiter.Wait()

			req, err := http.NewRequest("GET", treeURL, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}
			req.Header.Set("PRIVATE-TOKEN", c.tokens[c.currentToken])
			req.Header.Set("Accept", "application/json")

			resp, err = c.httpClient.Do(req)
			if err != nil {
				lastErr = err
				continue
			}

			c.updateRateLimitFromHeaders(resp.Header, correlationID)

			if resp.StatusCode == 429 {
				_ = c.WaitForRateLimit(resp, correlationID)
				resp.Body.Close()
				lastErr = fmt.Errorf("rate limited")
				continue
			}

			if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadGateway {
				resp.Body.Close()
				return nil, fmt.Errorf("%w: project %d", ErrGitLabRepositoryUnavailable, projectID)
			}

			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return nil, fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(bodyBytes))
			}

			break
		}

		if resp == nil {
			if lastErr != nil {
				return nil, fmt.Errorf("failed to fetch repository tree for project %d: %w", projectID, lastErr)
			}
			return nil, fmt.Errorf("failed to fetch repository tree for project %d", projectID)
		}

		var nodes []*GitLabTreeNode
		if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode repository tree: %w", err)
		}

		nextPage := resp.Header.Get("X-Next-Page")
		resp.Body.Close()

		allNodes = append(allNodes, nodes...)

		if nextPage == "" {
			break
		}

		nextPageInt, err := strconv.Atoi(nextPage)
		if err != nil || nextPageInt <= 0 {
			helper.LogInfo(ctx, "Invalid X-Next-Page value '%s', stopping pagination", nextPage)
			break
		}
		page = nextPageInt
	}

	return allNodes, nil
}

// GetFileContentRaw fetches raw file content from a blob SHA.
func (c *GitLabClient) GetFileContentRaw(projectID int, sha string, correlationID string) (string, error) {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitLabClient.GetFileContentRaw",
		CorrelationID: correlationID,
	}

	if projectID <= 0 {
		return "", fmt.Errorf("invalid project ID: %d", projectID)
	}
	if sha == "" {
		return "", fmt.Errorf("blob SHA is required")
	}

	blobURL := fmt.Sprintf("%s/api/v4/projects/%d/repository/blobs/%s/raw", c.baseURL, projectID, sha)

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		c.rateLimiter.Wait()

		req, err := http.NewRequest("GET", blobURL, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("PRIVATE-TOKEN", c.tokens[c.currentToken])

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		c.updateRateLimitFromHeaders(resp.Header, correlationID)

		if resp.StatusCode == 429 {
			_ = c.WaitForRateLimit(resp, correlationID)
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limited")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return "", fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(bodyBytes))
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("failed to read blob body: %w", err)
		}

		return string(bodyBytes), nil
	}

	if lastErr != nil {
		return "", fmt.Errorf("failed to fetch blob raw content after retries: %w", lastErr)
	}

	helper.LogInfo(ctx, "Failed to fetch blob raw content after retries (project=%d, sha=%s)", projectID, sha)
	return "", fmt.Errorf("failed to fetch blob raw content after retries")
}

// SearchCode searches GitLab for code matching the query using basic search
func (c *GitLabClient) SearchCode(query string, correlationID string) ([]*GitLabBlobResult, error) {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitLabClient.SearchCode",
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

	var allResults []*GitLabBlobResult
	page := 1
	perPage := 100 // GitLab max per page

	for {
		// Build search URL
		searchURL := fmt.Sprintf("%s/api/v4/search?scope=blobs&search=%s&page=%d&per_page=%d",
			c.baseURL,
			url.QueryEscape(query),
			page,
			perPage,
		)

		helper.LogInfo(ctx, "Requesting GitLab search (page %d): %s", page, searchURL)

		// Perform search with retry logic
		var results []*GitLabBlobResult
		var err error

		for attempt := 0; attempt < 3; attempt++ {
			results, err = c.executeSearch(searchURL, correlationID)
			if err == nil {
				break
			}

			// Retry on errors with exponential backoff
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			helper.LogInfo(ctx, "GitLab search failed, retrying in %v (attempt %d/3)", backoff, attempt+1)
			time.Sleep(backoff)
		}

		if err != nil {
			helper.LogError(ctx, "GitLab search failed after retries", err)
			return allResults, fmt.Errorf("GitLab search failed after retries: %w", err)
		}

		if len(results) == 0 {
			helper.LogInfo(ctx, "No more results found on page %d", page)
			break
		}

		helper.LogInfo(ctx, "Found %d results on page %d", len(results), page)
		allResults = append(allResults, results...)

		// Check if this was the last page
		if len(results) < perPage {
			helper.LogInfo(ctx, "Reached last page %d", page)
			break
		}

		page++

		// Add delay between pages to be polite to the API
		time.Sleep(1 * time.Second)
	}

	helper.LogInfo(ctx, "GitLab search completed: found %d total results", len(allResults))
	return allResults, nil
}

// executeSearch performs a single search request to GitLab
func (c *GitLabClient) executeSearch(searchURL string, correlationID string) ([]*GitLabBlobResult, error) {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitLabClient.executeSearch",
		CorrelationID: correlationID,
	}

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Set("PRIVATE-TOKEN", c.tokens[c.currentToken])
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle rate limiting
	if resp.StatusCode == 429 {
		helper.LogInfo(ctx, "GitLab API rate limit exceeded (HTTP 429)")
		if err := c.WaitForRateLimit(resp, correlationID); err != nil {
			return nil, fmt.Errorf("rate limit wait failed: %w", err)
		}
		return nil, fmt.Errorf("rate limited, retry needed")
	}

	// Check for other errors
	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		helper.LogError(ctx, "GitLab API returned non-200 status", fmt.Errorf("status: %d, body: %s", resp.StatusCode, string(bodyBytes)))
		return nil, fmt.Errorf("GitLab API returned status %d", resp.StatusCode)
	}

	// Update rate limit info from headers
	c.updateRateLimitFromHeaders(resp.Header, correlationID)

	// Parse response
	var results []*GitLabBlobResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return results, nil
}

// GetProject retrieves project details from GitLab
func (c *GitLabClient) GetProject(projectID int, correlationID string) (*GitLabProject, error) {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitLabClient.GetProject",
		CorrelationID: correlationID,
	}

	c.rateLimiter.Wait()

	projectURL := fmt.Sprintf("%s/api/v4/projects/%d", c.baseURL, projectID)
	helper.LogInfo(ctx, "Fetching project details: %s", projectURL)

	req, err := http.NewRequest("GET", projectURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", c.tokens[c.currentToken])
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("project not found")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitLab API returned status %d", resp.StatusCode)
	}

	// Update rate limit
	c.updateRateLimitFromHeaders(resp.Header, correlationID)

	var project GitLabProject
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("failed to decode project: %w", err)
	}

	return &project, nil
}

// updateRateLimitFromHeaders extracts rate limit info from response headers
func (c *GitLabClient) updateRateLimitFromHeaders(headers http.Header, correlationID string) {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitLabClient.updateRateLimitFromHeaders",
		CorrelationID: correlationID,
	}

	// GitLab uses RateLimit-* headers
	remainingStr := headers.Get("RateLimit-Remaining")
	resetStr := headers.Get("RateLimit-Reset")

	if remainingStr != "" && resetStr != "" {
		remaining, err1 := strconv.Atoi(remainingStr)
		resetUnix, err2 := strconv.ParseInt(resetStr, 10, 64)

		if err1 == nil && err2 == nil {
			resetTime := time.Unix(resetUnix, 0)
			c.rateLimiter.UpdateQuota(remaining, resetTime)
			helper.LogInfo(ctx, "GitLab API rate limit: %d remaining, resets at %v", remaining, resetTime)
		}
	}
}

// WaitForRateLimit handles HTTP 429 responses using Retry-After header
func (c *GitLabClient) WaitForRateLimit(resp *http.Response, correlationID string) error {
	ctx := helper.LogContext{
		ServiceName:   "scraper-service",
		Operation:     "GitLabClient.WaitForRateLimit",
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

	// Check RateLimit-Reset header
	resetStr := resp.Header.Get("RateLimit-Reset")
	if resetStr != "" {
		if resetUnix, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
			resetTime := time.Unix(resetUnix, 0)
			if resetTime.After(time.Now()) {
				waitDuration := time.Until(resetTime)
				helper.LogInfo(ctx, "Rate limited, waiting until reset at %v (%v)", resetTime, waitDuration)
				time.Sleep(waitDuration)
				return nil
			}
		}
	}

	// Default wait
	helper.LogInfo(ctx, "Rate limited, waiting 60 seconds (default)")
	time.Sleep(60 * time.Second)
	return nil
}

// RotateToken switches to the next available GitLab token
func (c *GitLabClient) RotateToken() {
	if len(c.tokens) <= 1 {
		helper.LogInfo(helper.LogContext{
			ServiceName: "scraper-service",
			Operation:   "GitLabClient.RotateToken",
		}, "Only one token available, cannot rotate")
		return
	}

	c.currentToken = (c.currentToken + 1) % len(c.tokens)
	helper.LogInfo(helper.LogContext{
		ServiceName: "scraper-service",
		Operation:   "GitLabClient.RotateToken",
	}, "Rotating to token %d/%d", c.currentToken+1, len(c.tokens))
}

// CheckRateLimit queries the current rate limit status
// Note: GitLab doesn't have a dedicated rate limit endpoint like GitHub
// Rate limits are only visible in response headers
func (c *GitLabClient) CheckRateLimit() (int, time.Time, error) {
	// Make a lightweight API call to check rate limit
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v4/version", c.baseURL), nil)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", c.tokens[c.currentToken])
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Extract rate limit from headers
	remainingStr := resp.Header.Get("RateLimit-Remaining")
	resetStr := resp.Header.Get("RateLimit-Reset")

	if remainingStr == "" || resetStr == "" {
		return 0, time.Time{}, fmt.Errorf("rate limit headers not found")
	}

	remaining, err := strconv.Atoi(remainingStr)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed to parse remaining: %w", err)
	}

	resetUnix, err := strconv.ParseInt(resetStr, 10, 64)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed to parse reset time: %w", err)
	}

	resetTime := time.Unix(resetUnix, 0)
	c.rateLimiter.UpdateQuota(remaining, resetTime)

	return remaining, resetTime, nil
}
