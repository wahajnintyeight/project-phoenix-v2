package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"project-phoenix/v2/internal/model"
)

var llmProviderHTTPClient = &http.Client{Timeout: 90 * time.Second}

var openAICompatibleLLMProviders = map[string]string{
	"huggingface": "https://router.huggingface.co/v1/chat/completions",
	"xai":         "https://api.x.ai/v1/chat/completions",
	"mistral":     "https://api.mistral.ai/v1/chat/completions",
	"deepseek":    "https://api.deepseek.com/chat/completions",
	"gemini":      "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions",
}

// IsSupportedLLMProvider reports whether the service can send requests to provider.
func IsSupportedLLMProvider(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai", "anthropic", "groq", "openrouter", "ollama":
		return true
	default:
		_, ok := openAICompatibleLLMProviders[strings.ToLower(strings.TrimSpace(provider))]
		return ok
	}
}

func (s *LLMService) sendOpenAICompatibleChatCompletion(req model.ChatCompletionRequest, provider, endpoint string) (*model.ChatCompletionResponse, error) {
	payload, err := json.Marshal(struct {
		Model       string              `json:"model"`
		Messages    []model.ChatMessage `json:"messages"`
		Temperature float64             `json:"temperature"`
		MaxTokens   int                 `json:"max_tokens"`
		Stream      bool                `json:"stream"`
	}{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s request: %w", provider, err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create %s request: %w", provider, err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := s.client
	if client == nil {
		client = llmProviderHTTPClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call %s: %w", provider, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return nil, fmt.Errorf("%s API returned status %d (failed to read error response: %w)", provider, resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("%s API returned status %d: %s", provider, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var completion struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&completion); err != nil {
		return nil, fmt.Errorf("failed to parse %s response: %w", provider, err)
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("%s returned no completion choices", provider)
	}

	content := completion.Choices[0].Message.Content
	usage := model.UsageInfo{
		PromptTokens:     completion.Usage.PromptTokens,
		CompletionTokens: completion.Usage.CompletionTokens,
		TotalTokens:      completion.Usage.TotalTokens,
	}
	if usage.TotalTokens == 0 {
		usage.PromptTokens = calculateTokens(req.Messages)
		usage.CompletionTokens = len(content) / 4
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	responseID := completion.ID
	if responseID == "" {
		responseID = generateResponseID()
	}
	responseModel := completion.Model
	if responseModel == "" {
		responseModel = req.Model
	}
	return &model.ChatCompletionResponse{
		ID:    responseID,
		Model: responseModel,
		Message: model.ChatMessage{
			Role:    "assistant",
			Content: content,
		},
		Usage:     usage,
		CreatedAt: time.Now(),
	}, nil
}
