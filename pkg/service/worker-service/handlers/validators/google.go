package validators

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/pkg/helper"
)

// GoogleValidator validates Google AI API keys
type GoogleValidator struct {
	*BaseValidator
}

// NewGoogleValidator creates a new Google validator
func NewGoogleValidator(debugMode bool) *GoogleValidator {
	return &GoogleValidator{
		BaseValidator: NewBaseValidator(debugMode),
	}
}

// GetProviderName returns the provider name
func (v *GoogleValidator) GetProviderName() string {
	return model.ProviderGoogle
}

// Validate validates a Google AI API key and detects free-tier keys via rate-limit probing
func (v *GoogleValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-3.5-flash:generateContent?key=%s",
		keyValue,
	)

	payload := `{
		"contents": [
			{
				"parts": [
					{
						"text": "PING"
					}
				]
			}
		],
		"generationConfig": {
			"maxOutputTokens": 1
    	}
	}`

	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		return model.StatusError, nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	status, err := v.ExecuteRequestWithRetry(req, correlationID)
	if err != nil {
		return status, nil, err
	}

	if status != model.StatusValid {
		return status, nil, nil
	}

	isFreeTier := v.detectFreeTier(keyValue, correlationID)
	credits := map[string]interface{}{
		"free_tier": isFreeTier,
	}

	if isFreeTier {
		return model.StatusValidNoCredits, credits, nil
	}

	return model.StatusValid, credits, nil
}

// detectFreeTier sends a rapid follow-up request. Free keys have a 2 RPM limit,
// so an immediate second request that fails with 429/503 indicates a free-tier key.
func (v *GoogleValidator) detectFreeTier(keyValue string, correlationID string) bool {
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "GoogleValidator.detectFreeTier",
		CorrelationID: correlationID,
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-3-flash-preview:generateContent?key=%s",
		keyValue,
	)

	payload := `{
		"contents": [
			{
				"parts": [
					{
						"text": "PING"
					}
				]
			}
		],
		"generationConfig": {
			"maxOutputTokens": 1
		}
	}`

	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		helper.LogError(ctx, "Failed to create rate-limit probe request", err)
		return false
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := v.HTTPClient.Do(req)
	if err != nil {
		helper.LogError(ctx, "Rate-limit probe request failed", sanitizeHTTPError(err))
		return false
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	helper.LogInfo(ctx, "Rate-limit probe response: HTTP %d", resp.StatusCode)

	if resp.StatusCode == 429 || resp.StatusCode == 503 {
		helper.LogInfo(ctx, "Google key appears to be free tier (rate limited on second request)")
		return true
	}

	if strings.Contains(strings.ToLower(bodyStr), "overloaded") ||
		strings.Contains(strings.ToLower(bodyStr), "resource has been exhausted") ||
		strings.Contains(strings.ToLower(bodyStr), "quota exceeded") {
		helper.LogInfo(ctx, "Google key appears to be free tier (quota/overload message)")
		return true
	}

	return false
}
