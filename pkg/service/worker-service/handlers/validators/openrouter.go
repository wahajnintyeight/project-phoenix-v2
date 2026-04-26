package validators

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/pkg/helper"
)

// OpenRouterValidator validates OpenRouter API keys
type OpenRouterValidator struct {
	*BaseValidator
}

// NewOpenRouterValidator creates a new OpenRouter validator
func NewOpenRouterValidator(debugMode bool) *OpenRouterValidator {
	return &OpenRouterValidator{
		BaseValidator: NewBaseValidator(debugMode),
	}
}

// GetProviderName returns the provider name
func (v *OpenRouterValidator) GetProviderName() string {
	return model.ProviderOpenRouter
}

// Validate validates an OpenRouter API key and returns credits info
func (v *OpenRouterValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "OpenRouterValidator.Validate",
		CorrelationID: correlationID,
	}

	url := "https://openrouter.ai/api/v1/credits"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return model.StatusError, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+keyValue)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.HTTPClient.Do(req)
	if err != nil {
		helper.LogError(ctx, "HTTP request error", err)
		return model.StatusError, nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		helper.LogError(ctx, "Failed to read response body", err)
		return model.StatusError, nil, err
	}

	if v.DebugMode {
		v.logResponse(req, resp)
		helper.LogDebug("  Body: %s", string(bodyBytes))
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return model.StatusInvalid, nil, nil
	}

	if resp.StatusCode != 200 {
		return model.StatusError, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var creditsResp struct {
		Data struct {
			TotalCredits *float64 `json:"total_credits"`
			TotalUsage   *float64 `json:"total_usage"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &creditsResp); err != nil {
		helper.LogError(ctx, "Failed to parse credits response", err)
		return model.StatusError, nil, err
	}

	if creditsResp.Data.TotalCredits == nil {
		helper.LogError(ctx, "OpenRouter API returned 200 but missing total_credits field", nil)
		return model.StatusError, nil, fmt.Errorf("missing total_credits in response")
	}

	totalCredits := *creditsResp.Data.TotalCredits
	totalUsage := float64(0)
	if creditsResp.Data.TotalUsage != nil {
		totalUsage = *creditsResp.Data.TotalUsage
	}

	credits := map[string]interface{}{
		"total_credits": totalCredits,
		"total_usage":   totalUsage,
		"checked_at":    time.Now(),
	}

	status := model.StatusValid
	if totalCredits <= 0 {
		status = model.StatusValidNoCredits
		helper.LogInfo(ctx, "OpenRouter key has no credits remaining")
	}

	helper.LogInfo(ctx, "OpenRouter key validated: credits=%.2f, usage=%.2f, status=%s",
		totalCredits, totalUsage, status)

	return status, credits, nil
}
