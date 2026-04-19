package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
	Error    string                 `json:"error,omitempty"`
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
		status, err := testOpenAIKey(keyValue, model)
		return buildResult(provider, status, nil, err)
	case "Anthropic":
		status, err := testAnthropicKey(keyValue, model)
		return buildResult(provider, status, nil, err)
	case "Google":
		status, err := testGoogleKey(keyValue)
		return buildResult(provider, status, nil, err)
	case "OpenRouter":
		status, credits, err := testOpenRouterKey(keyValue)
		return buildResult(provider, status, credits, err)
	default:
		return KeyTestResult{
			Provider: provider,
			Status:   "Error",
			Error:    fmt.Sprintf("unsupported provider: %s", provider),
		}
	}
}

func buildResult(provider, status string, credits map[string]interface{}, err error) KeyTestResult {
	r := KeyTestResult{Provider: provider, Status: status, Credits: credits}
	if err != nil {
		r.Error = err.Error()
		if status == "" {
			r.Status = "Error"
		}
	}
	return r
}

// testOpenAIKey validates an OpenAI key. Uses the provided model or defaults to gpt-4o-mini.
func testOpenAIKey(keyValue, model string) (string, error) {
	if model == "" {
		model = "gpt-4o-mini"
	}

	body, _ := json.Marshal(map[string]interface{}{
		"model":      model,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
	})

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return "Error", err
	}
	req.Header.Set("Authorization", "Bearer "+keyValue)
	req.Header.Set("Content-Type", "application/json")

	return doProviderRequest(req)
}

// testAnthropicKey validates an Anthropic key. Uses the provided model or defaults to claude-haiku-4-5-20251001.
func testAnthropicKey(keyValue, model string) (string, error) {
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}

	body, _ := json.Marshal(map[string]interface{}{
		"model":      model,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
	})

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(body))
	if err != nil {
		return "Error", err
	}
	req.Header.Set("X-Api-Key", keyValue)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	return doProviderRequest(req)
}

// testGoogleKey validates a Google AI key via the lightweight models list endpoint.
func testGoogleKey(keyValue string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", keyValue)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "Error", err
	}
	return doProviderRequest(req)
}

// testOpenRouterKey validates an OpenRouter key and returns credit info.
func testOpenRouterKey(keyValue string) (string, map[string]interface{}, error) {
	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/credits", nil)
	if err != nil {
		return "Error", nil, err
	}
	req.Header.Set("Authorization", "Bearer "+keyValue)
	req.Header.Set("Content-Type", "application/json")

	resp, err := keyTesterHTTPClient.Do(req)
	if err != nil {
		return "Error", nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "Error", nil, err
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return "Invalid", nil, nil
	}
	if resp.StatusCode != 200 {
		return "Error", nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var creditsResp struct {
		Data struct {
			TotalCredits *float64 `json:"total_credits"`
			TotalUsage   *float64 `json:"total_usage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &creditsResp); err != nil {
		return "Error", nil, err
	}
	if creditsResp.Data.TotalCredits == nil {
		return "Error", nil, fmt.Errorf("missing total_credits in response")
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
	return status, credits, nil
}

// doProviderRequest executes a one-shot request and maps the HTTP status to a key status string.
func doProviderRequest(req *http.Request) (string, error) {
	resp, err := keyTesterHTTPClient.Do(req)
	if err != nil {
		return "Error", err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == 200:
		return "Valid", nil
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return "Invalid", nil
	default:
		return "Error", fmt.Errorf("provider returned HTTP %d", resp.StatusCode)
	}
}
