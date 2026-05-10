package validators

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/pkg/helper"
)

// HuggingFaceValidator validates Hugging Face Hub access tokens (e.g. hf_..., api_org_...).
type HuggingFaceValidator struct {
	*BaseValidator
}

// NewHuggingFaceValidator creates a new Hugging Face validator.
func NewHuggingFaceValidator(debugMode bool) *HuggingFaceValidator {
	return &HuggingFaceValidator{
		BaseValidator: NewBaseValidator(debugMode),
	}
}

// GetProviderName returns the provider name.
func (v *HuggingFaceValidator) GetProviderName() string {
	return model.ProviderHuggingFace
}

// whoamiV2Response is a subset of https://huggingface.co/api/whoami-v2 JSON.
type whoamiV2Response struct {
	Name     string `json:"name"`
	Fullname string `json:"fullname"`
	Type     string `json:"type"`
}

const (
	huggingFaceWhoamiURL     = "https://huggingface.co/api/whoami-v2"
	defaultHFRouterChatURL   = "https://router.huggingface.co/v1/chat/completions"
	defaultHFRouterTestModel = "moonshotai/Kimi-K2.6"
)

// Validate checks the token with Hub whoami-v2, then with Inference Router chat completions.
func (v *HuggingFaceValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "HuggingFaceValidator.Validate",
		CorrelationID: correlationID,
	}

	whoamiStatus, credits, err := v.fetchWhoamiWithRetry(keyValue, correlationID)
	if err != nil {
		return model.StatusError, nil, err
	}
	if whoamiStatus != model.StatusValid {
		return whoamiStatus, nil, nil
	}

	routerStatus, err := v.routerChatCompletionPing(keyValue, correlationID)
	if err != nil {
		return model.StatusError, credits, err
	}

	if routerStatus != model.StatusValid {
		helper.LogInfo(ctx, "whoami succeeded but router chat returned status %s", routerStatus)
		return routerStatus, credits, nil
	}

	credits["router_chat_ok"] = true
	credits["router_model"] = hfRouterTestModel()
	credits["checked_at"] = time.Now()

	return model.StatusValid, credits, nil
}

func (v *HuggingFaceValidator) fetchWhoamiWithRetry(keyValue, correlationID string) (string, map[string]interface{}, error) {
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "HuggingFaceValidator.fetchWhoamiWithRetry",
		CorrelationID: correlationID,
	}

	maxRetries := 3
	retryDelay := 2 * time.Second
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			helper.LogInfo(ctx, "whoami retry attempt %d/%d after %v", attempt, maxRetries, retryDelay)
			time.Sleep(retryDelay)
		}

		req, err := http.NewRequest("GET", huggingFaceWhoamiURL, nil)
		if err != nil {
			return model.StatusError, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+keyValue)

		resp, err := v.HTTPClient.Do(req)
		if err != nil {
			lastErr = err
			helper.LogError(ctx, "whoami HTTP error", err)
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if v.DebugMode {
			v.logResponse(req, resp)
			helper.LogDebug("  whoami body: %s", string(body))
		}

		if readErr != nil {
			lastErr = readErr
			helper.LogError(ctx, "whoami read body error", readErr)
			continue
		}

		status := v.DetermineStatusFromResponse(resp)
		helper.LogInfo(ctx, "Hugging Face whoami: HTTP %d → %s", resp.StatusCode, status)

		if resp.StatusCode >= 500 && resp.StatusCode < 600 && attempt < maxRetries {
			continue
		}

		if status != model.StatusValid {
			return status, nil, nil
		}

		var w whoamiV2Response
		if err := json.Unmarshal(body, &w); err != nil {
			helper.LogError(ctx, "whoami JSON parse error", err)
			credits := map[string]interface{}{"checked_at": time.Now()}
			return model.StatusValid, credits, nil
		}

		credits := map[string]interface{}{
			"hub_user":     w.Name,
			"fullname":     w.Fullname,
			"account_type": w.Type,
			"checked_at":   time.Now(),
		}
		return model.StatusValid, credits, nil
	}

	if lastErr != nil {
		helper.LogError(ctx, "whoami max retries exceeded", lastErr)
		return model.StatusError, nil, lastErr
	}
	return model.StatusError, nil, helper.NewError("whoami max retries exceeded")
}

func hfRouterChatURL() string {
	if u := os.Getenv("HF_ROUTER_CHAT_URL"); u != "" {
		return u
	}
	return defaultHFRouterChatURL
}

func hfRouterTestModel() string {
	if m := os.Getenv("HF_ROUTER_TEST_MODEL"); m != "" {
		return m
	}
	return defaultHFRouterTestModel
}

// routerChatCompletionPing calls Inference Router with a tiny non-streaming completion (mirrors HF docs curl; stream=false for simple validation).
func (v *HuggingFaceValidator) routerChatCompletionPing(keyValue, correlationID string) (string, error) {
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "HuggingFaceValidator.routerChatCompletionPing",
		CorrelationID: correlationID,
	}

	payload := map[string]interface{}{
		"model": hfRouterTestModel(),
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
		"max_tokens": 1,
		"stream":     false,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return model.StatusError, err
	}

	req, err := http.NewRequest("POST", hfRouterChatURL(), bytes.NewReader(bodyBytes))
	if err != nil {
		return model.StatusError, err
	}
	req.Header.Set("Authorization", "Bearer "+keyValue)
	req.Header.Set("Content-Type", "application/json")

	status, err := v.ExecuteRequestWithRetry(req, correlationID)
	helper.LogInfo(ctx, "Hugging Face router chat: status=%s", status)
	return status, err
}
