package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// KeyTestRequest is the request body for the /validate-key endpoint.
type KeyTestRequest struct {
	KeyValue string `json:"key_value"`
	Provider string `json:"provider"`
	Model    string `json:"model,omitempty"` // optional override
}

// KeyTestResult holds the result for a single provider test.
type KeyTestResult struct {
	Provider string                 `json:"provider"`
	Status   string                 `json:"status"`
	Credits  map[string]interface{} `json:"credits,omitempty"`
	Response string                 `json:"response,omitempty"`
	Error    string                 `json:"error,omitempty"`
}

type providerResult struct {
	Status   string
	Response string
	Credits  map[string]interface{}
	Err      error
}

// keyTesterHTTPClient is a package-level client shared for test requests.
var keyTesterHTTPClient = &http.Client{Timeout: 20 * time.Second}

// HandleValidateKey handles POST /validate-key.
// It accepts a key_value, provider, and optional model. No data is persisted.
// Requires sessionId header for authentication.
func HandleValidateKey(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("sessionId")
	if sessionID == "" {
		http.Error(w, `{"error":"missing sessionId header"}`, http.StatusUnauthorized)
		return
	}

	var req KeyTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.KeyValue == "" || req.Provider == "" {
		http.Error(w, `{"error":"key_value and provider are required"}`, http.StatusBadRequest)
		return
	}

	log.Printf("[key-tester] Testing key for provider=%s (session=%s)", req.Provider, sessionID)

	result := testKey(req.KeyValue, req.Provider, req.Model)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":   1009,
		"result": result,
	})
}

// testKey dispatches to the correct provider validator and returns a KeyTestResult.
func testKey(keyValue, provider, model string) KeyTestResult {
	switch provider {
	case "OpenAI":
		result := testOpenAIKey(keyValue, model)
		return buildResult(provider, result)
	case "Anthropic":
		result := testAnthropicKey(keyValue, model)
		return buildResult(provider, result)
	case "Google":
		result := testGoogleKey(keyValue, model)
		return buildResult(provider, result)
	case "OpenRouter":
		result := testOpenRouterKey(keyValue, model)
		return buildResult(provider, result)
	default:
		return KeyTestResult{
			Provider: provider,
			Status:   "Error",
			Error:    fmt.Sprintf("unsupported provider: %s", provider),
		}
	}
}

func buildResult(provider string, result providerResult) KeyTestResult {
	r := KeyTestResult{
		Provider: provider,
		Status:   result.Status,
		Credits:  result.Credits,
		Response: result.Response,
	}
	if result.Err != nil {
		r.Error = result.Err.Error()
		if result.Status == "" {
			r.Status = "Error"
		}
	}
	return r
}

// testOpenAIKey validates an OpenAI key. Uses the provided model or defaults to gpt-4o-mini.
func testOpenAIKey(keyValue, model string) providerResult {
	if model == "" {
		model = "gpt-4o-mini"
	}

	body, err := json.Marshal(map[string]interface{}{
		"model":      model,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
	})
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+keyValue)
	req.Header.Set("Content-Type", "application/json")

	return doProviderRequest(req)
}

// testAnthropicKey validates an Anthropic key. Uses the provided model or defaults to claude-haiku-4-5-20251001.
func testAnthropicKey(keyValue, model string) providerResult {
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}

	body, err := json.Marshal(map[string]interface{}{
		"model":      model,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
	})
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(body))
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}
	req.Header.Set("X-Api-Key", keyValue)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	return doProviderRequest(req)
}

// testGoogleKey validates a Google AI key.
// If a model is supplied, it tests actual content generation against that model.
// Otherwise it falls back to the lightweight models list endpoint.
func testGoogleKey(keyValue, model string) providerResult {
	var (
		req *http.Request
		err error
	)

	if model == "" {
		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", keyValue)
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return providerResult{Status: "Error", Err: err}
		}
		return doProviderRequest(req)
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		model,
		keyValue,
	)
	body, err := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{{"text": "ping"}},
			},
		},
	})
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}

	req, err = http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}
	req.Header.Set("Content-Type", "application/json")

	return doProviderRequest(req)
}

// testOpenRouterKey validates an OpenRouter key.
// If model is provided, it validates with a chat completion curl call.
// Otherwise it falls back to credits endpoint.
func testOpenRouterKey(keyValue, model string) providerResult {
	if strings.TrimSpace(model) != "" {
		return testOpenRouterModelWithCurl(keyValue, model)
	}

	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/credits", nil)
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+keyValue)
	req.Header.Set("Content-Type", "application/json")

	resp, err := keyTesterHTTPClient.Do(req)
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}

	responseMessage := extractProviderResponse(bodyBytes)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return providerResult{Status: "Invalid", Response: responseMessage}
	}
	if resp.StatusCode != 200 {
		return providerResult{
			Status:   "Error",
			Response: responseMessage,
			Err:      fmt.Errorf("unexpected status %d", resp.StatusCode),
		}
	}

	var creditsResp struct {
		Data struct {
			TotalCredits *float64 `json:"total_credits"`
			TotalUsage   *float64 `json:"total_usage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &creditsResp); err != nil {
		return providerResult{Status: "Error", Response: responseMessage, Err: err}
	}
	if creditsResp.Data.TotalCredits == nil {
		return providerResult{
			Status:   "Error",
			Response: responseMessage,
			Err:      fmt.Errorf("missing total_credits in response"),
		}
	}

	totalCredits := *creditsResp.Data.TotalCredits
	totalUsage := float64(0)
	if creditsResp.Data.TotalUsage != nil {
		totalUsage = *creditsResp.Data.TotalUsage
	}

	credits := map[string]interface{}{
		"total_credits": totalCredits,
		"total_usage":   totalUsage,
	}

	status := "Valid"
	if totalCredits <= 0 {
		status = "ValidNoCredits"
	}
	return providerResult{Status: status, Credits: credits, Response: responseMessage}
}

func testOpenRouterModelWithCurl(keyValue, model string) providerResult {
	payload := map[string]interface{}{
		"messages": []map[string]string{
			{
				"content": "You are a helpful assistant.",
				"role":    "system",
			},
			{
				"content": "What is the capital of France?",
				"role":    "user",
			},
		},
		"max_tokens":  150,
		"model":       model,
		"temperature": 0.7,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"curl",
		"-sS",
		"-X", "POST",
		"https://openrouter.ai/api/v1/chat/completions",
		"-H", "Authorization: Bearer "+keyValue,
		"-H", "Content-Type: application/json",
		"-d", string(body),
		"-w", "\n%{http_code}",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return providerResult{Status: "Error", Err: fmt.Errorf("curl timeout after 25s")}
		}
		return providerResult{Status: "Error", Err: fmt.Errorf("curl failed: %w, output: %s", err, strings.TrimSpace(string(output)))}
	}

	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return providerResult{Status: "Error", Err: fmt.Errorf("empty curl response")}
	}

	lastNewline := strings.LastIndex(raw, "\n")
	if lastNewline == -1 {
		responseMessage := extractProviderResponse([]byte(raw))
		return providerResult{Status: "Error", Response: responseMessage, Err: fmt.Errorf("unable to parse curl status code")}
	}

	bodyPart := strings.TrimSpace(raw[:lastNewline])
	statusPart := strings.TrimSpace(raw[lastNewline+1:])

	statusCode := 0
	if _, scanErr := fmt.Sscanf(statusPart, "%d", &statusCode); scanErr != nil {
		responseMessage := extractProviderResponse([]byte(bodyPart))
		return providerResult{Status: "Error", Response: responseMessage, Err: fmt.Errorf("invalid curl status code: %s", statusPart)}
	}

	responseMessage := extractProviderResponse([]byte(bodyPart))

	switch {
	case statusCode == 200:
		return providerResult{Status: "Valid", Response: responseMessage}
	case statusCode == 401 || statusCode == 403:
		return providerResult{Status: "Invalid", Response: responseMessage}
	default:
		return providerResult{
			Status:   "Error",
			Response: responseMessage,
			Err:      fmt.Errorf("provider returned HTTP %d", statusCode),
		}
	}
}

// doProviderRequest executes a one-shot request and maps the HTTP status to a key status string.
func doProviderRequest(req *http.Request) providerResult {
	resp, err := keyTesterHTTPClient.Do(req)
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}

	responseMessage := extractProviderResponse(bodyBytes)

	switch {
	case resp.StatusCode == 200:
		return providerResult{Status: "Valid", Response: responseMessage}
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return providerResult{Status: "Invalid", Response: responseMessage}
	default:
		return providerResult{
			Status:   "Error",
			Response: responseMessage,
			Err:      fmt.Errorf("provider returned HTTP %d", resp.StatusCode),
		}
	}
}

func extractProviderResponse(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return trimmed
	}

	if message := nestedProviderMessage(parsed); message != "" {
		return message
	}

	return trimmed
}

func nestedProviderMessage(payload map[string]interface{}) string {
	if errorValue, ok := payload["error"]; ok {
		switch typed := errorValue.(type) {
		case string:
			return typed
		case map[string]interface{}:
			if message, ok := typed["message"].(string); ok && strings.TrimSpace(message) != "" {
				return message
			}
			if status, ok := typed["status"].(string); ok && strings.TrimSpace(status) != "" {
				return status
			}
		}
	}

	if message, ok := payload["message"].(string); ok && strings.TrimSpace(message) != "" {
		return message
	}

	if status, ok := payload["status"].(string); ok && strings.TrimSpace(status) != "" {
		return status
	}

	if promptFeedback, ok := payload["promptFeedback"].(map[string]interface{}); ok {
		if blockReason, ok := promptFeedback["blockReason"].(string); ok && strings.TrimSpace(blockReason) != "" {
			return blockReason
		}
	}

	return ""
}
