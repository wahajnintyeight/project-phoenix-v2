package validators

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"project-phoenix/v2/internal/model"
)

// MiMoValidator validates MiMo API keys.
type MiMoValidator struct {
	*BaseValidator
}

func NewMiMoValidator(debugMode bool) *MiMoValidator {
	return &MiMoValidator{BaseValidator: NewBaseValidator(debugMode)}
}

func (v *MiMoValidator) GetProviderName() string {
	return model.ProviderMiMo
}

func (v *MiMoValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
	modelName := "mimo-v2.5"
	body, err := json.Marshal(map[string]interface{}{
		"model":      modelName,
		"max_tokens": 1,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
	})
	if err != nil {
		return model.StatusError, nil, err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.xiaomimimo.com/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return model.StatusError, nil, err
	}

	req.Header.Set("api-key", strings.TrimSpace(keyValue))
	req.Header.Set("Content-Type", "application/json")

	status, err := v.ExecuteRequestWithRetry(req, correlationID)
	return status, nil, err
}
