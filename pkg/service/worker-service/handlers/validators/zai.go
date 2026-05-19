package validators

import (
	"bytes"
	"encoding/json"
	"net/http"

	"project-phoenix/v2/internal/model"
)

// ZAIValidator validates Z.AI (Zhipu AI) API keys
type ZAIValidator struct {
	*BaseValidator
}

// NewZAIValidator creates a new Z.AI validator
func NewZAIValidator(debugMode bool) *ZAIValidator {
	return &ZAIValidator{
		BaseValidator: NewBaseValidator(debugMode),
	}
}

// GetProviderName returns the provider name
func (v *ZAIValidator) GetProviderName() string {
	return model.ProviderZAI
}

// Validate validates a Z.AI API key
func (v *ZAIValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
	url := "https://api.z.ai/api/paas/v4/chat/completions"

	requestBody := map[string]interface{}{
		"model":       "glm-5.1",
		"max_tokens":  1,
		"stream":      false,
		"temperature": 1,
		"messages": []map[string]string{
			{"role": "user", "content": "PING"},
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
	req.Header.Set("Accept-Language", "en-US,en")

	status, err := v.ExecuteRequestWithRetry(req, correlationID)
	return status, nil, err
}
