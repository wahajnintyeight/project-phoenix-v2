package validators

import (
	"net/http"

	"project-phoenix/v2/internal/model"
)

// XAIValidator validates xAI API keys without consuming tokens.
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
	req, err := http.NewRequest(http.MethodGet, "https://api.x.ai/v1/models", nil)
	if err != nil {
		return model.StatusError, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+keyValue)
	status, err := v.ExecuteRequestWithRetry(req, correlationID)
	return status, nil, err
}
