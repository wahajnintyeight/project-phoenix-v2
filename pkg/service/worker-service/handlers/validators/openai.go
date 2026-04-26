package validators

import (
	"bytes"
	"encoding/json"
	"net/http"

	"project-phoenix/v2/internal/model"
)

// OpenAIValidator validates OpenAI API keys
type OpenAIValidator struct {
	*BaseValidator
}

// NewOpenAIValidator creates a new OpenAI validator
func NewOpenAIValidator(debugMode bool) *OpenAIValidator {
	return &OpenAIValidator{
		BaseValidator: NewBaseValidator(debugMode),
	}
}

// GetProviderName returns the provider name
func (v *OpenAIValidator) GetProviderName() string {
	return model.ProviderOpenAI
}

// Validate validates an OpenAI API key
func (v *OpenAIValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
	url := "https://api.openai.com/v1/chat/completions"

	requestBody := map[string]interface{}{
		"model":      "gpt-5.4",
		"max_tokens": 100,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
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
