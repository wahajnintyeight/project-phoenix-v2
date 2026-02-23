package model

import "time"

// LLMAPIConfig represents a stored LLM API configuration
type LLMAPIConfig struct {
	ID              string    `json:"id" bson:"_id,omitempty"`
	Name            string    `json:"name" bson:"name"`               // User-friendly name
	Provider        string    `json:"provider" bson:"provider"`       // openai, anthropic, groq, openrouter, ollama
	Model           string    `json:"model" bson:"model"`             // gpt-4, claude-3-opus, etc.
	APIKey          string    `json:"apiKey,omitempty" bson:"apiKey"` // Encrypted API key (not returned in list)
	EncryptedAPIKey string    `json:"-" bson:"encryptedApiKey"`       // Stored encrypted
	IsActive        bool      `json:"isActive" bson:"isActive"`       // Whether this config is active
	CreatedBy       string    `json:"createdBy" bson:"createdBy"`     // User who created it
	CreatedAt       time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt" bson:"updatedAt"`
}

// LLMAPIConfigRequest represents a request to create/update LLM API config
type LLMAPIConfigRequest struct {
	Name     string `json:"name" bson:"name"`
	Provider string `json:"provider" bson:"provider"`
	Model    string `json:"model" bson:"model"`
	APIKey   string `json:"apiKey" bson:"apiKey"`
	IsActive bool   `json:"isActive" bson:"isActive"`
}

// LLMAPIConfigResponse represents the response (without sensitive data)
type LLMAPIConfigResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	IsActive  bool      `json:"isActive"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ATSScanRequest represents a request to scan resume with stored API config
type ATSScanRequest struct {
	APIID          string  `json:"api_id" bson:"api_id"`                   // ID of stored LLM API config
	ResumeText     string  `json:"resume_text" bson:"resume_text"`         // Resume content
	JobDescription string  `json:"job_description" bson:"job_description"` // Job description
	Temperature    float64 `json:"temperature" bson:"temperature"`         // Optional, defaults to 0.7
	MaxTokens      int     `json:"maxTokens" bson:"maxTokens"`             // Optional, defaults to 2000
}

// ToResponse converts LLMAPIConfig to response format (without API key)
func (c *LLMAPIConfig) ToResponse() LLMAPIConfigResponse {
	return LLMAPIConfigResponse{
		ID:        c.ID,
		Name:      c.Name,
		Provider:  c.Provider,
		Model:     c.Model,
		IsActive:  c.IsActive,
		CreatedBy: c.CreatedBy,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}
