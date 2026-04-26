package validators

import (
	"bytes"
	"encoding/json"
	"net/http"

	"project-phoenix/v2/internal/model"
)

// MoonshotValidator validates Moonshot AI API keys
type MoonshotValidator struct {
	*BaseValidator
}

// NewMoonshotValidator creates a new Moonshot validator
func NewMoonshotValidator(debugMode bool) *MoonshotValidator {
	return &MoonshotValidator{
		BaseValidator: NewBaseValidator(debugMode),
	}
}

// GetProviderName returns the provider name
func (v *MoonshotValidator) GetProviderName() string {
	return model.ProviderMoonshot
}

// Validate validates a Moonshot AI API key
func (v *MoonshotValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
	url := "https://api.moonshot.ai/v1/chat/completions"

	requestBody := map[string]interface{}{
		"model":       "kimi-k2-0905-preview",
		"temperature": 0.3,
		"max_tokens":  100,
		"top_p":       1,
		"stream":      false, // Disable streaming for validation
		"messages": []map[string]string{
			{"role": "system", "content": "You are a helpful assistant."},
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
