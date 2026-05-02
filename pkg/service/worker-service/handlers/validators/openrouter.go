package validators

import (
	"bytes"
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
// First validates the key by calling the chat completions API, then fetches credits
func (v *OpenRouterValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "OpenRouterValidator.Validate",
		CorrelationID: correlationID,
	}

	// Step 1: Validate the key by calling chat completions API
	chatURL := "https://openrouter.ai/api/v1/chat/completions"
	chatPayload := []byte(`{
		"messages": [
			{"content": "You are a helpful assistant.", "role": "system"},
			{"content": "What is the capital of France?", "role": "user"}
		],
		"max_tokens": 32000,
		"model": "~openai/gpt-latest",
		"temperature": 0.7
	}`)

	chatReq, err := http.NewRequest("POST", chatURL, bytes.NewBuffer(chatPayload))
	if err != nil {
		return model.StatusError, nil, err
	}

	chatReq.Header.Set("Authorization", "Bearer "+keyValue)
	chatReq.Header.Set("Content-Type", "application/json")

	chatResp, err := v.HTTPClient.Do(chatReq)
	if err != nil {
		helper.LogError(ctx, "Chat completions HTTP request error", err)
		return model.StatusError, nil, err
	}
	defer chatResp.Body.Close()

	chatBodyBytes, err := io.ReadAll(chatResp.Body)
	if err != nil {
		helper.LogError(ctx, "Failed to read chat completions response body", err)
		return model.StatusError, nil, err
	}

	if v.DebugMode {
		v.logResponse(chatReq, chatResp)
		helper.LogDebug("  Chat Body: %s", string(chatBodyBytes))
	}

	// Check if the key is invalid
	if chatResp.StatusCode == 401 || chatResp.StatusCode == 403 || chatResp.StatusCode == 402 || chatResp.StatusCode == 404 {
		return model.StatusInvalid, nil, nil
	}

	// Only proceed to credits check if chat completions returned 200
	if chatResp.StatusCode != 200 {
		return model.StatusError, nil, fmt.Errorf("chat completions unexpected status code: %d", chatResp.StatusCode)
	}

	helper.LogInfo(ctx, "OpenRouter key validated via chat completions API")

	// Step 2: Fetch credits information
	creditsURL := "https://openrouter.ai/api/v1/credits"

	creditsReq, err := http.NewRequest("GET", creditsURL, nil)
	if err != nil {
		return model.StatusError, nil, err
	}

	creditsReq.Header.Set("Authorization", "Bearer "+keyValue)
	creditsReq.Header.Set("Content-Type", "application/json")

	creditsResp, err := v.HTTPClient.Do(creditsReq)
	if err != nil {
		helper.LogError(ctx, "Credits HTTP request error", err)
		return model.StatusError, nil, err
	}
	defer creditsResp.Body.Close()

	creditsBodyBytes, err := io.ReadAll(creditsResp.Body)
	if err != nil {
		helper.LogError(ctx, "Failed to read credits response body", err)
		return model.StatusError, nil, err
	}

	if v.DebugMode {
		v.logResponse(creditsReq, creditsResp)
		helper.LogDebug("  Credits Body: %s", string(creditsBodyBytes))
	}

	if creditsResp.StatusCode != 200 {
		return model.StatusError, nil, fmt.Errorf("credits unexpected status code: %d", creditsResp.StatusCode)
	}

	var creditsData struct {
		Data struct {
			TotalCredits *float64 `json:"total_credits"`
			TotalUsage   *float64 `json:"total_usage"`
		} `json:"data"`
	}

	if err := json.Unmarshal(creditsBodyBytes, &creditsData); err != nil {
		helper.LogError(ctx, "Failed to parse credits response", err)
		return model.StatusError, nil, err
	}

	if creditsData.Data.TotalCredits == nil {
		helper.LogError(ctx, "OpenRouter API returned 200 but missing total_credits field", nil)
		return model.StatusError, nil, fmt.Errorf("missing total_credits in response")
	}

	totalCredits := *creditsData.Data.TotalCredits
	totalUsage := float64(0)
	if creditsData.Data.TotalUsage != nil {
		totalUsage = *creditsData.Data.TotalUsage
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
