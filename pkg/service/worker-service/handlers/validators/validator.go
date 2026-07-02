package validators

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/pkg/helper"
)

// ValidationResult contains the result of a key validation
type ValidationResult struct {
	Status  string
	Credits map[string]interface{}
	Error   error
}

// Validator interface defines the contract for provider-specific validators
type Validator interface {
	// Validate validates an API key and returns status, credits, and error
	Validate(keyValue string, correlationID string) (string, map[string]interface{}, error)

	// GetProviderName returns the provider name this validator handles
	GetProviderName() string
}

// BaseValidator provides common functionality for all validators
type BaseValidator struct {
	HTTPClient *http.Client
	DebugMode  bool
}

// NewBaseValidator creates a new base validator with common settings
func NewBaseValidator(debugMode bool) *BaseValidator {
	return &BaseValidator{
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		DebugMode: debugMode,
	}
}

// ExecuteRequestWithRetry executes HTTP request with retry logic for 5xx errors
func (b *BaseValidator) ExecuteRequestWithRetry(req *http.Request, correlationID string) (string, error) {
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "BaseValidator.ExecuteRequestWithRetry",
		CorrelationID: correlationID,
	}

	maxRetries := 3
	retryDelay := 2 * time.Second

	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			helper.LogInfo(ctx, "Retry attempt %d/%d after %v", attempt, maxRetries, retryDelay)
			time.Sleep(retryDelay)

			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					helper.LogError(ctx, "Failed to reset request body", err)
					return model.StatusError, err
				}
				req.Body = body
			}
		}

		resp, lastErr = b.HTTPClient.Do(req)
		if lastErr != nil {
			lastErr = sanitizeHTTPError(lastErr)
			helper.LogError(ctx, "HTTP request error", lastErr)
			return model.StatusError, lastErr
		}

		// Log response if debug mode is enabled
		if b.DebugMode {
			b.logResponse(req, resp)
		}

		// Determine status from response
		status := b.DetermineStatusFromResponse(resp)
		helper.LogInfo(ctx, "Provider API response: HTTP %d, determined status: %s", resp.StatusCode, status)

		// Retry on 5xx errors
		if resp.StatusCode >= 500 && resp.StatusCode < 600 && attempt < maxRetries {
			helper.LogInfo(ctx, "Received %d status, retrying...", resp.StatusCode)
			resp.Body.Close()
			continue
		}

		resp.Body.Close()
		return status, nil
	}

	// All retries exhausted
	if lastErr != nil {
		helper.LogError(ctx, "Max retries exceeded", lastErr)
		return model.StatusError, lastErr
	}

	return model.StatusError, helper.NewError("max retries exceeded")
}

func sanitizeHTTPError(err error) error {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}

	safeErr := *urlErr
	safeErr.URL = sanitizeURL(urlErr.URL)
	safeErr.Err = sanitizeHTTPError(urlErr.Err)
	return &safeErr
}

func sanitizeURL(rawURL string) string {
	if parsed, parseErr := url.Parse(rawURL); parseErr == nil {
		q := parsed.Query()
		if q.Has("key") {
			q.Set("key", "[REDACTED]")
			parsed.RawQuery = q.Encode()
			return parsed.String()
		}
	}
	return rawURL
}

// DetermineStatusFromResponse determines key status from HTTP response
func (b *BaseValidator) DetermineStatusFromResponse(resp *http.Response) string {
	switch {
	case resp.StatusCode == 200:
		return model.StatusValid
	case resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 429 || resp.StatusCode == 400:
		return model.StatusInvalid
	default:
		return model.StatusError
	}
}

// logResponse logs HTTP response details in debug mode
func (b *BaseValidator) logResponse(req *http.Request, resp *http.Response) {
	helper.LogDebug("Provider API Response:")
	helper.LogDebug("  URL: %s", req.URL.String())
	helper.LogDebug("  Status Code: %d", resp.StatusCode)
	helper.LogDebug("  Status: %s", resp.Status)
	helper.LogDebug("  Headers: %+v", resp.Header)
}
