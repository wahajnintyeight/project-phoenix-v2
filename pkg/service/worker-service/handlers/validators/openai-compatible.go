package validators

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"project-phoenix/v2/internal/model"
)

type OpenAICompatibleValidator struct {
	*BaseValidator
	provider string
	url      string
	model    string
	tokenKey string
}

func NewOpenAICompatibleValidator(provider, url, modelName, tokenKey string, debugMode bool) *OpenAICompatibleValidator {
	validator := &OpenAICompatibleValidator{
		BaseValidator: NewBaseValidator(debugMode),
		provider:      provider,
		url:           url,
		model:         modelName,
		tokenKey:      tokenKey,
	}

	if provider == model.ProviderMistral {
		validator.HTTPClient.Transport = newHTTP1OnlyValidatorTransport()
	}

	return validator
}

func (v *OpenAICompatibleValidator) GetProviderName() string {
	return v.provider
}

func (v *OpenAICompatibleValidator) Validate(keyValue, correlationID string) (string, map[string]interface{}, error) {
	payload := map[string]interface{}{
		"model":    v.model,
		"messages": []map[string]string{{"role": "user", "content": "Reply OK"}},
		v.tokenKey: 16,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return model.StatusError, nil, err
	}

	req, err := http.NewRequest(http.MethodPost, v.url, bytes.NewReader(body))
	if err != nil {
		return model.StatusError, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(keyValue))
	req.Header.Set("Content-Type", "application/json")

	status, err := v.ExecuteRequestWithRetry(req, correlationID)
	return status, nil, err
}
