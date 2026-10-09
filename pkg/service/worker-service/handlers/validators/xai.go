package validators

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"project-phoenix/v2/internal/model"
)

const xAIValidationModel = "grok-4.7"

// XAIValidator validates xAI API keys by making a minimal chat completion.
type XAIValidator struct {
	*BaseValidator
}

func NewXAIValidator(debugMode bool) *XAIValidator {
	return &XAIValidator{BaseValidator: NewBaseValidator(debugMode)}
}

func (v *XAIValidator) GetProviderName() string {
	return model.ProviderXAI
}

func (v *XAIValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
	payload := map[string]interface{}{
		"model": xAIValidationModel,
		"messages": []map[string]string{
			{"role": "user", "content": "PING"},
		},
		"max_tokens": 1,
		"stream":     false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return model.StatusError, nil, err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.x.ai/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return model.StatusError, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(keyValue))
	req.Header.Set("Content-Type", "application/json")
	status, err := v.ExecuteRequestWithRetry(req, correlationID)
	return status, nil, err
}
