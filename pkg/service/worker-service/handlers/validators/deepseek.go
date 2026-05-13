package validators

import (
	"bytes"
	"encoding/json"
	"net/http"

	"project-phoenix/v2/internal/model"
)

// DeepSeekValidator validates DeepSeek API keys
type DeepSeekValidator struct {
	*BaseValidator
}

// NewDeepSeekValidator creates a new DeepSeek validator
func NewDeepSeekValidator(debugMode bool) *DeepSeekValidator {
	return &DeepSeekValidator{
		BaseValidator: NewBaseValidator(debugMode),
	}
}

// GetProviderName returns the provider name
func (v *DeepSeekValidator) GetProviderName() string {
	return model.ProviderDeepSeek
}

// Validate validates a DeepSeek API key
func (v *DeepSeekValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
	url := "https://api.deepseek.com/v1/chat/completions"

	requestBody := map[string]interface{}{
		"model":      "deepseek-v4-flash",
		"max_tokens": 1,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
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

	req.Header.Set("Authorization", "Bearer "+keyValue)
	req.Header.Set("Content-Type", "application/json")

	status, err := v.ExecuteRequestWithRetry(req, correlationID)
	return status, nil, err
}
