package model

import "time"

// ChatMessage represents a single message in a chat conversation
type ChatMessage struct {
	Role    string `json:"role" bson:"role"`       // "system", "user", or "assistant"
	Content string `json:"content" bson:"content"` // The message content
}

// ChatCompletionRequest represents a request for chat-based LLM completion
type ChatCompletionRequest struct {
	Model       string        `json:"model" bson:"model"`             // LLM model to use
	Messages    []ChatMessage `json:"messages" bson:"messages"`       // Conversation history
	Temperature float64       `json:"temperature" bson:"temperature"` // Sampling temperature (0-1)
	MaxTokens   int           `json:"maxTokens" bson:"maxTokens"`     // Maximum tokens to generate
	Stream      bool          `json:"stream" bson:"stream"`           // Whether to stream responses
	Type        PromptType    `json:"type" bson:"type"`               // Prompt type (e.g., ATS_SCAN)
	Provider    string        `json:"provider" bson:"provider"`       // LLM provider (e.g., "openai", "anthropic")
	APIKey      string        `json:"apiKey" bson:"apiKey"`           // API key for the provider
}

// TextCompletionRequest represents a request for text completion
type TextCompletionRequest struct {
	Model       string  `json:"model" bson:"model"`             // LLM model to use
	Prompt      string  `json:"prompt" bson:"prompt"`           // Input prompt
	Temperature float64 `json:"temperature" bson:"temperature"` // Sampling temperature (0-1)
	MaxTokens   int     `json:"maxTokens" bson:"maxTokens"`     // Maximum tokens to generate
}

// ChatCompletionResponse represents the response from a chat completion
type ChatCompletionResponse struct {
	ID        string      `json:"id" bson:"id"`               // Unique response ID
	Model     string      `json:"model" bson:"model"`         // Model used
	Message   ChatMessage `json:"message" bson:"message"`     // Generated message
	Usage     UsageInfo   `json:"usage" bson:"usage"`         // Token usage information
	CreatedAt time.Time   `json:"createdAt" bson:"createdAt"` // Response timestamp
}

// TextCompletionResponse represents the response from a text completion
type TextCompletionResponse struct {
	ID        string    `json:"id" bson:"id"`               // Unique response ID
	Model     string    `json:"model" bson:"model"`         // Model used
	Text      string    `json:"text" bson:"text"`           // Generated text
	Usage     UsageInfo `json:"usage" bson:"usage"`         // Token usage information
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"` // Response timestamp
}

// UsageInfo represents token usage statistics
type UsageInfo struct {
	PromptTokens     int `json:"promptTokens" bson:"promptTokens"`         // Tokens in prompt
	CompletionTokens int `json:"completionTokens" bson:"completionTokens"` // Tokens in completion
	TotalTokens      int `json:"totalTokens" bson:"totalTokens"`           // Total tokens used
}

// LLMTestConnectionRequest represents a request to test LLM API connection
type LLMTestConnectionRequest struct {
	Provider string `json:"provider" bson:"provider"` // LLM provider (e.g., "openai", "anthropic", "openrouter")
	Model    string `json:"model" bson:"model"`       // Model to test (e.g., "gpt-4")
	APIKey   string `json:"apiKey" bson:"apiKey"`     // API key to test
}

// OpenRouterModel represents a model from OpenRouter API
type OpenRouterModel struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Created       int64                  `json:"created"`
	ContextLength int                    `json:"context_length"`
	Pricing       OpenRouterPricing      `json:"pricing"`
	Architecture  OpenRouterArchitecture `json:"architecture"`
	Description   string                 `json:"description"`
}

// OpenRouterPricing represents pricing information for a model
type OpenRouterPricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
	Request    string `json:"request"`
	Image      string `json:"image"`
}

// OpenRouterArchitecture represents model architecture details
type OpenRouterArchitecture struct {
	Modality         string   `json:"modality"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
	Tokenizer        string   `json:"tokenizer"`
	InstructType     string   `json:"instruct_type"`
}

// OpenRouterModelsResponse represents the response from OpenRouter models API
type OpenRouterModelsResponse struct {
	Data []OpenRouterModel `json:"data"`
}

// FetchModelsRequest represents a request to fetch models for a provider
type FetchModelsRequest struct {
	Provider string `json:"provider" bson:"provider"` // Provider to fetch models for
	APIKey   string `json:"apiKey" bson:"apiKey"`     // API key for the provider
}
