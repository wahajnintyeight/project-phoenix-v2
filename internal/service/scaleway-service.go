package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type scalewayChatRequest struct {
	Model           string            `json:"model"`
	Messages        []scalewayMessage `json:"messages"`
	MaxTokens       int               `json:"max_tokens,omitempty"`
	Temperature     float64           `json:"temperature,omitempty"`
	TopP            float64           `json:"top_p,omitempty"`
	PresencePenalty float64           `json:"presence_penalty,omitempty"`
	ResponseFormat  map[string]string `json:"response_format,omitempty"`
	Stream          bool              `json:"stream"`
}

type scalewayMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type scalewayChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Text string `json:"text"`
	} `json:"choices"`
}

func (s *LLMService) generateTextWithScaleway(ctx context.Context, prompt string) (string, error) {
	endpoint := getEnvOrDefault("SCALEWAY_CHAT_COMPLETIONS_URL", "https://api.scaleway.ai/438fe6f8-0589-4cae-83c1-25b9de302813/v1/chat/completions")
	apiKey := getEnvOrDefault("SCW_SECRET_KEY", "")
	model := getEnvOrDefault("SCALEWAY_MODEL", "voxtral-small-24b-2507")

	if apiKey == "" {
		return "", fmt.Errorf("SCW_SECRET_KEY not configured")
	}

	reqBody := scalewayChatRequest{
		Model: model,
		Messages: []scalewayMessage{
			{Role: "system", Content: "You are a professional cricket commentator."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:       220,
		Temperature:     0.4,
		TopP:            0.95,
		PresencePenalty: 0,
		ResponseFormat:  map[string]string{"type": "text"},
		Stream:          false,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal scaleway request: %w", err)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create scaleway request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("scaleway request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("scaleway API status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out scalewayChatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("failed to decode scaleway response: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("scaleway response has no choices")
	}

	content := strings.TrimSpace(out.Choices[0].Message.Content)
	if content == "" {
		content = strings.TrimSpace(out.Choices[0].Delta.Content)
	}
	if content == "" {
		content = strings.TrimSpace(out.Choices[0].Text)
	}
	if content == "" {
		return "", fmt.Errorf("scaleway response content is empty")
	}

	return content, nil
}
