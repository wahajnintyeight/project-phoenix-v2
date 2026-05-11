package validators

import (
	"fmt"
	"net/http"
	"strings"

	"project-phoenix/v2/internal/model"
)

// GoogleValidator validates Google AI API keys
type GoogleValidator struct {
	*BaseValidator
}

// NewGoogleValidator creates a new Google validator
func NewGoogleValidator(debugMode bool) *GoogleValidator {
	return &GoogleValidator{
		BaseValidator: NewBaseValidator(debugMode),
	}
}

// GetProviderName returns the provider name
func (v *GoogleValidator) GetProviderName() string {
	return model.ProviderGoogle
}

// Validate validates a Google AI API key
func (v *GoogleValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-3-flash-preview:generateContent?key=%s",
		keyValue,
	)

	payload := `{
		"contents": [
			{
				"parts": [
					{
						"text": "PING"
					}
				]
			}
		],
		"generationConfig": {
			"maxOutputTokens": 32000,
			"thinkingConfig": {
				"thinkingLevel": "MEDIUM"
			}
    	}
	}`

	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		return model.StatusError, nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	status, err := v.ExecuteRequestWithRetry(req, correlationID)
	return status, nil, err
}
