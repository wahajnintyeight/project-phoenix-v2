package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"project-phoenix/v2/internal/model"
	"strings"
	"time"
)

// MultimodalLLMService handles multimodal LLM requests with native vision support
type MultimodalLLMService struct {
	client *http.Client
}

// NewMultimodalLLMService creates a new multimodal LLM service
func NewMultimodalLLMService() *MultimodalLLMService {
	return &MultimodalLLMService{
		client: &http.Client{
			Timeout: 120 * time.Second, // Longer timeout for vision models
		},
	}
}

// SendChatCompletion sends a chat completion request with native multimodal support
func (s *MultimodalLLMService) SendChatCompletion(req model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	// Validate request
	if req.Provider == "" {
		return nil, fmt.Errorf("provider is required")
	}
	if req.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Check if request contains images
	hasImages := s.containsImages(req.Messages)

	// Route to appropriate handler based on provider and content type
	switch strings.ToLower(req.Provider) {
	case "openrouter":
		return s.sendOpenRouterRequest(req, hasImages)
	case "anthropic":
		return s.sendAnthropicRequest(req, hasImages)
	case "openai":
		return s.sendOpenAIRequest(req, hasImages)
	case "ollama":
		return s.sendOllamaRequest(req, hasImages)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", req.Provider)
	}
}

// containsImages checks if any message contains image content
func (s *MultimodalLLMService) containsImages(messages []model.ChatMessage) bool {
	for _, msg := range messages {
		if parts, ok := msg.Content.([]model.ContentPart); ok {
			for _, part := range parts {
				if part.Type == "image_url" {
					return true
				}
			}
		}
	}
	return false
}

// sendOpenRouterRequest sends request to OpenRouter (OpenAI-compatible format)
func (s *MultimodalLLMService) sendOpenRouterRequest(req model.ChatCompletionRequest, hasImages bool) (*model.ChatCompletionResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Build OpenRouter-compatible request
	payload := map[string]interface{}{
		"model":       req.Model,
		"messages":    s.formatMessagesForOpenRouter(req.Messages),
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", req.APIKey))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "https://project-phoenix.local")
	httpReq.Header.Set("X-Title", "Project Phoenix")

	log.Printf("Sending %s request to OpenRouter with model %s",
		map[bool]string{true: "multimodal", false: "text"}[hasImages], req.Model)

	// Send request
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Errorf("OpenRouter API error (status %d): %s", resp.StatusCode, string(body))

		// If it's a credit/rate-limit error and we have images, we can't fallback
		// (Scaleway doesn't support vision)
		if hasImages && (resp.StatusCode == 429 || resp.StatusCode == 402) {
			return nil, fmt.Errorf("OpenRouter vision API unavailable (credits/rate-limit) and no fallback available for multimodal: %w", errMsg)
		}

		// For text-only requests, try Scaleway fallback
		if !hasImages && (resp.StatusCode == 429 || resp.StatusCode == 402) {
			log.Printf("OpenRouter credit/rate-limit detected, falling back to Scaleway for text-only request")
			textPrompt := s.buildTextPromptFromMessages(req.Messages)
			scalewayResp, fallbackErr := s.generateTextWithScaleway(ctx, textPrompt)
			if fallbackErr == nil {
				return &model.ChatCompletionResponse{
					ID:    "scaleway-fallback",
					Model: "scaleway-llama",
					Message: model.ChatMessage{
						Role:    "assistant",
						Content: scalewayResp,
					},
					Usage: model.UsageInfo{
						PromptTokens:     len(textPrompt) / 4,
						CompletionTokens: len(scalewayResp) / 4,
						TotalTokens:      (len(textPrompt) + len(scalewayResp)) / 4,
					},
					CreatedAt: time.Now(),
				}, nil
			}
			log.Printf("Scaleway fallback failed: %v", fallbackErr)
		}

		return nil, errMsg
	}

	// Parse response
	var openRouterResp struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &openRouterResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(openRouterResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	// Build response
	return &model.ChatCompletionResponse{
		ID:    openRouterResp.ID,
		Model: openRouterResp.Model,
		Message: model.ChatMessage{
			Role:    "assistant",
			Content: openRouterResp.Choices[0].Message.Content,
		},
		Usage: model.UsageInfo{
			PromptTokens:     openRouterResp.Usage.PromptTokens,
			CompletionTokens: openRouterResp.Usage.CompletionTokens,
			TotalTokens:      openRouterResp.Usage.TotalTokens,
		},
		CreatedAt: time.Now(),
	}, nil
}

// sendAnthropicRequest sends request to Anthropic (native format)
func (s *MultimodalLLMService) sendAnthropicRequest(req model.ChatCompletionRequest, hasImages bool) (*model.ChatCompletionResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Extract system message
	systemMessage := ""
	messages := []model.ChatMessage{}
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if content, ok := msg.Content.(string); ok {
				systemMessage = content
			}
		} else {
			messages = append(messages, msg)
		}
	}

	// Build Anthropic-compatible request
	payload := map[string]interface{}{
		"model":       req.Model,
		"messages":    s.formatMessagesForAnthropic(messages),
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	}

	if systemMessage != "" {
		payload["system"] = systemMessage
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", req.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	log.Printf("Sending %s request to Anthropic with model %s",
		map[bool]string{true: "multimodal", false: "text"}[hasImages], req.Model)

	// Send request
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Anthropic API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var anthropicResp struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract text content
	var content string
	for _, c := range anthropicResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	// Build response
	return &model.ChatCompletionResponse{
		ID:    anthropicResp.ID,
		Model: anthropicResp.Model,
		Message: model.ChatMessage{
			Role:    "assistant",
			Content: content,
		},
		Usage: model.UsageInfo{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
		CreatedAt: time.Now(),
	}, nil
}

// sendOpenAIRequest sends request to OpenAI (native format)
func (s *MultimodalLLMService) sendOpenAIRequest(req model.ChatCompletionRequest, hasImages bool) (*model.ChatCompletionResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Build OpenAI-compatible request
	payload := map[string]interface{}{
		"model":       req.Model,
		"messages":    s.formatMessagesForOpenRouter(req.Messages), // Same format as OpenRouter
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", req.APIKey))
	httpReq.Header.Set("Content-Type", "application/json")

	log.Printf("Sending %s request to OpenAI with model %s",
		map[bool]string{true: "multimodal", false: "text"}[hasImages], req.Model)

	// Send request
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response (same format as OpenRouter)
	var openAIResp struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	// Build response
	return &model.ChatCompletionResponse{
		ID:    openAIResp.ID,
		Model: openAIResp.Model,
		Message: model.ChatMessage{
			Role:    "assistant",
			Content: openAIResp.Choices[0].Message.Content,
		},
		Usage: model.UsageInfo{
			PromptTokens:     openAIResp.Usage.PromptTokens,
			CompletionTokens: openAIResp.Usage.CompletionTokens,
			TotalTokens:      openAIResp.Usage.TotalTokens,
		},
		CreatedAt: time.Now(),
	}, nil
}

// sendOllamaRequest sends request to Ollama (local models)
func (s *MultimodalLLMService) sendOllamaRequest(req model.ChatCompletionRequest, hasImages bool) (*model.ChatCompletionResponse, error) {
	// Ollama support depends on the specific model and API version
	// For now, return an error suggesting to use the text-only service
	return nil, fmt.Errorf("Ollama multimodal support not yet implemented - use LLMService for text-only Ollama requests")
}

// formatMessagesForOpenRouter formats messages for OpenRouter/OpenAI API
func (s *MultimodalLLMService) formatMessagesForOpenRouter(messages []model.ChatMessage) []map[string]interface{} {
	formatted := make([]map[string]interface{}, 0, len(messages))

	for _, msg := range messages {
		formattedMsg := map[string]interface{}{
			"role": msg.Role,
		}

		// Handle content based on type
		switch content := msg.Content.(type) {
		case string:
			// Simple text content
			formattedMsg["content"] = content
		case []model.ContentPart:
			// Multimodal content
			parts := make([]map[string]interface{}, 0, len(content))
			for _, part := range content {
				if part.Type == "text" {
					parts = append(parts, map[string]interface{}{
						"type": "text",
						"text": part.Text,
					})
				} else if part.Type == "image_url" && part.ImageURL != nil {
					parts = append(parts, map[string]interface{}{
						"type": "image_url",
						"image_url": map[string]interface{}{
							"url": part.ImageURL.URL,
						},
					})
				}
			}
			formattedMsg["content"] = parts
		default:
			// Fallback to string representation
			formattedMsg["content"] = fmt.Sprintf("%v", content)
		}

		formatted = append(formatted, formattedMsg)
	}

	return formatted
}

// formatMessagesForAnthropic formats messages for Anthropic API
func (s *MultimodalLLMService) formatMessagesForAnthropic(messages []model.ChatMessage) []map[string]interface{} {
	formatted := make([]map[string]interface{}, 0, len(messages))

	for _, msg := range messages {
		formattedMsg := map[string]interface{}{
			"role": msg.Role,
		}

		// Handle content based on type
		switch content := msg.Content.(type) {
		case string:
			// Simple text content
			formattedMsg["content"] = content
		case []model.ContentPart:
			// Multimodal content - Anthropic uses different format
			parts := make([]map[string]interface{}, 0, len(content))
			for _, part := range content {
				if part.Type == "text" {
					parts = append(parts, map[string]interface{}{
						"type": "text",
						"text": part.Text,
					})
				} else if part.Type == "image_url" && part.ImageURL != nil {
					// Anthropic expects base64 data in a specific format
					imageData := part.ImageURL.URL
					// Extract base64 data if it's a data URL
					if strings.HasPrefix(imageData, "data:image/") {
						dataParts := strings.SplitN(imageData, ",", 2)
						if len(dataParts) == 2 {
							// Extract media type
							mediaType := "image/jpeg" // default
							if strings.Contains(dataParts[0], "image/png") {
								mediaType = "image/png"
							} else if strings.Contains(dataParts[0], "image/webp") {
								mediaType = "image/webp"
							} else if strings.Contains(dataParts[0], "image/gif") {
								mediaType = "image/gif"
							}

							parts = append(parts, map[string]interface{}{
								"type": "image",
								"source": map[string]interface{}{
									"type":       "base64",
									"media_type": mediaType,
									"data":       dataParts[1],
								},
							})
						}
					}
				}
			}
			formattedMsg["content"] = parts
		default:
			// Fallback to string representation
			formattedMsg["content"] = fmt.Sprintf("%v", content)
		}

		formatted = append(formatted, formattedMsg)
	}

	return formatted
}

// buildTextPromptFromMessages converts messages to a simple text prompt (for fallback)
func (s *MultimodalLLMService) buildTextPromptFromMessages(messages []model.ChatMessage) string {
	var promptBuilder strings.Builder

	for _, msg := range messages {
		var contentStr string

		switch content := msg.Content.(type) {
		case string:
			contentStr = content
		case []model.ContentPart:
			for _, part := range content {
				if part.Type == "text" {
					contentStr += part.Text + " "
				} else if part.Type == "image_url" {
					contentStr += "[Image data omitted in fallback mode] "
				}
			}
		default:
			contentStr = fmt.Sprintf("%v", content)
		}

		switch msg.Role {
		case "system":
			promptBuilder.WriteString(fmt.Sprintf("System: %s\n\n", contentStr))
		case "user":
			promptBuilder.WriteString(fmt.Sprintf("User: %s\n\n", contentStr))
		case "assistant":
			promptBuilder.WriteString(fmt.Sprintf("Assistant: %s\n\n", contentStr))
		}
	}

	promptBuilder.WriteString("Assistant:")
	return promptBuilder.String()
}

// generateTextWithScaleway fallback for multimodal service (text-only)
// Delegates to the existing LLMService implementation
func (s *MultimodalLLMService) generateTextWithScaleway(ctx context.Context, prompt string) (string, error) {
	llmService := NewLLMService()
	return llmService.generateTextWithScaleway(ctx, prompt)
}
