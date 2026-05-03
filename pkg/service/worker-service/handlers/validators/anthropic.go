package validators

import (
	"bytes"
	"encoding/json"
	"net/http"

	"project-phoenix/v2/internal/model"
)

// AnthropicValidator validates Anthropic API keys
type AnthropicValidator struct {
	*BaseValidator
}

// NewAnthropicValidator creates a new Anthropic validator
func NewAnthropicValidator(debugMode bool) *AnthropicValidator {
	return &AnthropicValidator{
		BaseValidator: NewBaseValidator(debugMode),
	}
}

// GetProviderName returns the provider name
func (v *AnthropicValidator) GetProviderName() string {
	return model.ProviderAnthropic
}

// Validate validates an Anthropic API key
func (v *AnthropicValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
	url := "https://api.anthropic.com/v1/messages"

	requestBody := map[string]interface{}{
		"model":      "claude-opus-4-6",
		"max_tokens": 1024,
		"messages": []map[string]string{
			{"role": "user", "content": "test"},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return model.StatusError, nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return model.StatusError, nil, err
	}

	req.Header.Set("X-Api-Key", keyValue)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	status, err := v.ExecuteRequestWithRetry(req, correlationID)
	return status, nil, err
}
