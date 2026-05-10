package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"project-phoenix/v2/internal/model"

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

type openRouterModelsResult struct {
	Provider string                  `json:"provider"`
	Models   []model.OpenRouterModel `json:"models"`
	Count    int                     `json:"count"`
	Error    string                  `json:"error,omitempty"`
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

// HandleFetchOpenRouterModels handles GET /validate-key/openrouter-models.
// It returns the available OpenRouter models for a supplied API key.
func HandleFetchOpenRouterModels(w http.ResponseWriter, r *http.Request) {
	apiKey := strings.TrimSpace(r.URL.Query().Get("api_key"))
	if apiKey == "" {
		http.Error(w, `{"error":"api_key is required"}`, http.StatusBadRequest)
		return
	}

	result := fetchOpenRouterModels(apiKey)
	if result.Err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":   1009,
			"result": openRouterModelsResult{Provider: "openrouter", Error: result.Err.Error()},
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	var payload openRouterModelsResult
	if err := json.Unmarshal([]byte(result.Response), &payload); err != nil {
		http.Error(w, `{"error":"failed to encode models response"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":   1009,
		"result": payload,
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
	case "HuggingFace":
		result := testHuggingFaceKey(keyValue)
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
// If model is provided, it validates with a chat completion request.
// Otherwise it falls back to credits endpoint.
func testOpenRouterKey(keyValue, model string) providerResult {
	if strings.TrimSpace(model) != "" {
		return testOpenRouterModelWithRequest(keyValue, model)
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

// testHuggingFaceKey validates via Hub whoami-v2, then Inference Router chat completions (non-streaming ping).
func testHuggingFaceKey(keyValue string) providerResult {
	req, err := http.NewRequest("GET", "https://huggingface.co/api/whoami-v2", nil)
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+keyValue)

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

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
		return providerResult{Status: "Invalid", Response: responseMessage}
	case http.StatusOK:
		// continue to router test
	default:
		return providerResult{
			Status:   "Error",
			Response: responseMessage,
			Err:      fmt.Errorf("whoami returned HTTP %d", resp.StatusCode),
		}
	}

	var w struct {
		Name     string `json:"name"`
		Fullname string `json:"fullname"`
		Type     string `json:"type"`
	}
	credits := map[string]interface{}{}
	if err := json.Unmarshal(bodyBytes, &w); err == nil {
		credits["hub_user"] = w.Name
		credits["fullname"] = w.Fullname
		credits["account_type"] = w.Type
	}

	routerURL := "https://router.huggingface.co/v1/chat/completions"
	if u := os.Getenv("HF_ROUTER_CHAT_URL"); u != "" {
		routerURL = u
	}
	modelName := "moonshotai/Kimi-K2.6"
	if m := os.Getenv("HF_ROUTER_TEST_MODEL"); m != "" {
		modelName = m
	}

	chatPayload, err := json.Marshal(map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
		"max_tokens": 1,
		"stream":     false,
	})
	if err != nil {
		return providerResult{Status: "Error", Credits: credits, Err: err}
	}

	chatReq, err := http.NewRequest("POST", routerURL, bytes.NewReader(chatPayload))
	if err != nil {
		return providerResult{Status: "Error", Credits: credits, Err: err}
	}
	chatReq.Header.Set("Authorization", "Bearer "+keyValue)
	chatReq.Header.Set("Content-Type", "application/json")

	chatResp, err := keyTesterHTTPClient.Do(chatReq)
	if err != nil {
		return providerResult{Status: "Error", Credits: credits, Err: err}
	}
	defer chatResp.Body.Close()

	chatBody, err := io.ReadAll(chatResp.Body)
	if err != nil {
		return providerResult{Status: "Error", Credits: credits, Err: err}
	}

	chatMsg := extractProviderResponse(chatBody)
	combinedResponse := responseMessage
	if chatMsg != "" {
		combinedResponse = fmt.Sprintf("whoami: %s | router: %s", responseMessage, chatMsg)
	}

	switch chatResp.StatusCode {
	case http.StatusOK:
		credits["router_chat_ok"] = true
		credits["router_model"] = modelName
		return providerResult{Status: "Valid", Credits: credits, Response: combinedResponse}
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests, http.StatusBadRequest:
		return providerResult{Status: "Invalid", Credits: credits, Response: combinedResponse}
	default:
		return providerResult{
			Status:   "Error",
			Credits:  credits,
			Response: combinedResponse,
			Err:      fmt.Errorf("router chat returned HTTP %d", chatResp.StatusCode),
		}
	}
}

func fetchOpenRouterModels(apiKey string) providerResult {
	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := keyTesterHTTPClient.Do(req)
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return providerResult{Status: "Invalid", Response: extractProviderResponse(bodyBytes)}
	}
	if resp.StatusCode != 200 {
		return providerResult{
			Status:   "Error",
			Response: extractProviderResponse(bodyBytes),
			Err:      fmt.Errorf("provider returned HTTP %d", resp.StatusCode),
		}
	}

	var openRouterResp model.OpenRouterModelsResponse
	if err := json.Unmarshal(bodyBytes, &openRouterResp); err != nil {
		return providerResult{Status: "Error", Err: err}
	}

	resultBody, err := json.Marshal(openRouterModelsResult{
		Provider: "openrouter",
		Models:   openRouterResp.Data,
		Count:    len(openRouterResp.Data),
	})
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}

	return providerResult{Status: "Valid", Response: string(resultBody)}
}

func testOpenRouterModelWithRequest(keyValue, model string) providerResult {
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
		"max_tokens":  32000,
		"model":       model,
		"temperature": 0.7,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return providerResult{Status: "Error", Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+keyValue)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := keyTesterHTTPClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return providerResult{Status: "Error", Err: fmt.Errorf("request timeout after 25s")}
		}
		return providerResult{Status: "Error", Err: fmt.Errorf("request failed: %w", err)}
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
