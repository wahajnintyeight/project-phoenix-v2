package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"project-phoenix/v2/internal/model"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/teilomillet/gollm"
)

// LLMService handles interactions with LLM providers
type LLMService struct{}

var llmCreditExhaustedIndicators = []string{
	"insufficient_quota",
	"quota exceeded",
	"exceeded your current quota",
	"out of credits",
	"credit balance",
	"rate limit exceeded",
	"free-models-per-day",
	"status code 429",
	"payment required",
	"billing",
	"402",
}

var llmCreditFallbackMessages = []string{
	"AI analysis is temporarily unavailable due to exhausted credits. Falling back to metadata-based tracking.",
	"LLM quota is exhausted right now. Logging this screenshot with static activity analysis.",
	"No AI credits remaining for this cycle. Continuing with deterministic fallback insights.",
	"Provider credits are depleted. Recording a safe default summary until credits are restored.",
	"LLM billing limit reached. Using static analysis mode for this screenshot.",
}

// NewLLMService creates a new LLM service instance
func NewLLMService() *LLMService {
	return &LLMService{}
}

// SendChatCompletion sends a chat completion request to the specified LLM provider
func (s *LLMService) SendChatCompletion(req model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	ctx := context.Background()

	// Validate provider and API key
	if req.Provider == "" {
		return nil, fmt.Errorf("provider is required")
	}
	if req.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Configure LLM based on provider
	var llm gollm.LLM
	var err error

	switch strings.ToLower(req.Provider) {
	case "openai":
		llm, err = gollm.NewLLM(
			gollm.SetProvider("openai"),
			gollm.SetModel(req.Model),
			gollm.SetAPIKey(req.APIKey),
			gollm.SetMaxTokens(req.MaxTokens),
			gollm.SetTemperature(req.Temperature),
		)
	case "anthropic":
		llm, err = gollm.NewLLM(
			gollm.SetProvider("anthropic"),
			gollm.SetModel(req.Model),
			gollm.SetAPIKey(req.APIKey),
			gollm.SetMaxTokens(req.MaxTokens),
			gollm.SetTemperature(req.Temperature),
		)
	case "groq":
		llm, err = gollm.NewLLM(
			gollm.SetProvider("groq"),
			gollm.SetModel(req.Model),
			gollm.SetAPIKey(req.APIKey),
			gollm.SetMaxTokens(req.MaxTokens),
			gollm.SetTemperature(req.Temperature),
		)
	case "openrouter":
		llm, err = gollm.NewLLM(
			gollm.SetProvider("openrouter"),
			gollm.SetModel(req.Model),
			gollm.SetAPIKey(req.APIKey),
			gollm.SetMaxTokens(req.MaxTokens),
			gollm.SetTemperature(req.Temperature),
		)
	case "ollama":
		llm, err = gollm.NewLLM(
			gollm.SetProvider("ollama"),
			gollm.SetModel(req.Model),
			gollm.SetMaxTokens(req.MaxTokens),
			gollm.SetTemperature(req.Temperature),
		)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", req.Provider)
	}

	if err != nil {
		log.Printf("Error creating LLM instance: %v", err)
		return nil, fmt.Errorf("failed to create LLM instance: %w", err)
	}

	// Build the prompt from messages
	prompt := s.buildPromptFromMessages(req.Messages)

	// Generate response
	log.Printf("Sending request to %s with model %s", req.Provider, req.Model)
	response, err := llm.Generate(ctx, gollm.NewPrompt(prompt))
	if err != nil {
		if strings.EqualFold(req.Provider, "openrouter") && s.IsCreditExhaustedError(err) {
			log.Printf("OpenRouter credit/rate-limit detected, falling back to Scaleway: %v", err)
			scalewayResp, fallbackErr := s.generateTextWithScaleway(ctx, prompt)
			if fallbackErr == nil {
				response = scalewayResp
			} else {
				log.Printf("Scaleway fallback failed: %v", fallbackErr)
				return nil, fmt.Errorf("failed to generate response (openrouter and scaleway fallback failed): openrouter=%v, scaleway=%w", err, fallbackErr)
			}
		} else {
			log.Printf("Error generating response: %v", err)
			return nil, fmt.Errorf("failed to generate response: %w", err)
		}
	}

	// Parse usage information if available
	usage := model.UsageInfo{
		PromptTokens:     calculateTokens(req.Messages),
		CompletionTokens: len(response) / 4, // Rough estimate
		TotalTokens:      calculateTokens(req.Messages) + (len(response) / 4),
	}

	// Create response
	chatResponse := &model.ChatCompletionResponse{
		ID:    generateResponseID(),
		Model: req.Model,
		Message: model.ChatMessage{
			Role:    "assistant",
			Content: response,
		},
		Usage:     usage,
		CreatedAt: getCurrentTime(),
	}

	log.Printf("Successfully generated response from %s", req.Provider)
	return chatResponse, nil
}

// IsCreditExhaustedError checks whether an LLM error indicates quota/credit exhaustion.
func (s *LLMService) IsCreditExhaustedError(err error) bool {
	if err == nil {
		return false
	}

	errText := strings.ToLower(err.Error())
	for _, indicator := range llmCreditExhaustedIndicators {
		if strings.Contains(errText, indicator) {
			return true
		}
	}

	return false
}

// GetCreditExhaustedFallbackMessages returns static fallback messages for quota exhaustion scenarios.
func (s *LLMService) GetCreditExhaustedFallbackMessages() []string {
	msgs := make([]string, len(llmCreditFallbackMessages))
	copy(msgs, llmCreditFallbackMessages)
	return msgs
}

// GetCreditExhaustedFallbackMessage returns a deterministic fallback message based on seed context.
func (s *LLMService) GetCreditExhaustedFallbackMessage(seed string) string {
	msgs := s.GetCreditExhaustedFallbackMessages()
	if len(msgs) == 0 {
		return "AI analysis unavailable. Using metadata fallback."
	}
	if seed == "" {
		return msgs[0]
	}

	hash := 0
	for _, ch := range seed {
		hash += int(ch)
	}

	return msgs[hash%len(msgs)]
}

// buildPromptFromMessages converts chat messages to a single prompt string
func (s *LLMService) buildPromptFromMessages(messages []model.ChatMessage) string {
	var promptBuilder strings.Builder

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			promptBuilder.WriteString(fmt.Sprintf("System: %s\n\n", msg.Content))
		case "user":
			promptBuilder.WriteString(fmt.Sprintf("User: %s\n\n", msg.Content))
		case "assistant":
			promptBuilder.WriteString(fmt.Sprintf("Assistant: %s\n\n", msg.Content))
		}
	}

	// Add final prompt for assistant response
	promptBuilder.WriteString("Assistant:")

	return promptBuilder.String()
}

// ScanResumeWithJobDescription performs ATS scoring by comparing resume with job description
func (s *LLMService) ScanResumeWithJobDescription(req model.ChatCompletionRequest, resumeText, jobDescription string) (*model.ChatCompletionResponse, error) {
	// Ensure the type is ATS_SCAN
	if req.Type != model.ATS_SCAN {
		return nil, fmt.Errorf("invalid request type for resume scanning")
	}

	// Get the ATS_SCORE system prompt
	systemPrompt := model.GetATSPrompt(model.ATS_SCORE)

	// Replace placeholders in the system prompt
	systemPrompt = strings.ReplaceAll(systemPrompt, "{resume_text}", resumeText)
	systemPrompt = strings.ReplaceAll(systemPrompt, "{job_description}", jobDescription)

	// Build messages with the system prompt
	messages := []model.ChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: "Please analyze this resume against the job description and provide the ATS score in the specified JSON format.",
		},
	}

	// Update request with prepared messages
	req.Messages = messages

	// Send the request
	return s.SendChatCompletion(req)
}

// ParseATSScoreResponse parses the ATS score response from JSON string
func (s *LLMService) ParseATSScoreResponse(response string) (map[string]interface{}, error) {
	var result map[string]interface{}

	// Try to extract JSON from the response
	// Sometimes LLMs wrap JSON in markdown code blocks
	jsonStr := extractJSON(response)

	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ATS score response: %w", err)
	}

	return result, nil
}

// Helper functions

func calculateTokens(messages []model.ChatMessage) int {
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
	}
	// Rough estimate: 1 token ≈ 4 characters
	return totalChars / 4
}

func generateResponseID() string {
	return uuid.New().String()
}

func getCurrentTime() time.Time {
	return time.Now()
}

func extractJSON(text string) string {
	// Remove markdown code blocks if present
	text = strings.TrimSpace(text)

	// Check for ```json ... ``` pattern
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		if idx := strings.LastIndex(text, "```"); idx != -1 {
			text = text[:idx]
		}
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		if idx := strings.LastIndex(text, "```"); idx != -1 {
			text = text[:idx]
		}
	}

	return strings.TrimSpace(text)
}

// FetchOpenRouterModels fetches available models from OpenRouter API
func (s *LLMService) FetchOpenRouterModels(apiKey string) ([]model.OpenRouterModel, error) {
	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request
	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Content-Type", "application/json")

	// Make request
	log.Println("Fetching models from OpenRouter API")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter API returned status %d", resp.StatusCode)
	}

	// Parse response
	var modelsResp model.OpenRouterModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("Successfully fetched %d models from OpenRouter", len(modelsResp.Data))
	return modelsResp.Data, nil
}

// GenerateText generates text using the default LLM configuration
// This is a simplified method for quick text generation without full configuration
func (s *LLMService) GenerateText(prompt string) (string, error) {
	ctx := context.Background()

	// Use default configuration (can be made configurable via env vars)
	provider := getEnvOrDefault("LLM_PROVIDER", "openrouter")
	model := getEnvOrDefault("OPENROUTER_MODEL", "llama-3.3-70b-versatile")
	apiKey := getEnvOrDefault("OPENROUTER_API_KEY", "")

	if apiKey == "" {
		return "", fmt.Errorf("LLM_API_KEY not configured")
	}

	// Create LLM instance
	llm, err := gollm.NewLLM(
		gollm.SetProvider(provider),
		gollm.SetModel(model),
		gollm.SetAPIKey(apiKey),
		gollm.SetMaxTokens(150),
		gollm.SetTemperature(0.8),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create LLM instance: %w", err)
	}

	// Generate response
	response, err := llm.Generate(ctx, gollm.NewPrompt(prompt))
	if err != nil {
		// OpenRouter credit/rate-limit failures should fallback to Scaleway.
		if strings.EqualFold(provider, "openrouter") && s.IsCreditExhaustedError(err) {
			log.Printf("OpenRouter credit/rate-limit detected, falling back to Scaleway: %v", err)
			scalewayResp, fallbackErr := s.generateTextWithScaleway(ctx, prompt)
			if fallbackErr == nil {
				return scalewayResp, nil
			}
			return "", fmt.Errorf("failed to generate text (openrouter and scaleway fallback failed): openrouter=%v, scaleway=%w", err, fallbackErr)
		}
		return "", fmt.Errorf("failed to generate text: %w", err)
	}

	return response, nil
}

// getEnvOrDefault retrieves environment variable or returns default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
