package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/internal/service"
	"time"

	"github.com/google/uuid"
)

type GoLLMController struct {
	DB         db.DBInterface
	LLMService *service.LLMService
}

func (g *GoLLMController) GetCollectionName() string {
	return "gollm_requests"
}

// ChatCompletion handles chat-based LLM completion requests
func (g *GoLLMController) ChatCompletion(w http.ResponseWriter, r *http.Request) (int, interface{}, error) {
	var req model.ChatCompletionRequest

	decodeErr := json.NewDecoder(r.Body).Decode(&req)
	if decodeErr != nil {
		log.Println("Error decoding chat completion request:", decodeErr)
		return int(enum.ERROR), "Invalid request body", decodeErr
	}

	// Validate request
	if req.Model == "" {
		return int(enum.ERROR), "Model is required", nil
	}
	if len(req.Messages) == 0 {
		return int(enum.ERROR), "Messages array cannot be empty", nil
	}

	// Check if type is specified and inject system prompt
	if req.Type != "" {
		log.Printf("Processing request with type: %s", req.Type)

		// Handle ATS_SCAN type
		if req.Type == model.ATS_SCAN {
			// Get the ATS_SCORE prompt
			systemPromptTemplate := model.GetATSPrompt(model.ATS_SCORE)
			if systemPromptTemplate == "" {
				log.Println("Invalid ATS prompt type")
				return int(enum.ERROR), "Invalid ATS prompt type", nil
			}

			// Inject system prompt as first message if not already present
			if len(req.Messages) == 0 || req.Messages[0].Role != "system" {
				systemMessage := model.ChatMessage{
					Role:    "system",
					Content: systemPromptTemplate,
				}
				req.Messages = append([]model.ChatMessage{systemMessage}, req.Messages...)
			} else {
				// Replace existing system message with the template
				req.Messages[0].Content = systemPromptTemplate
			}

			log.Println("ATS_SCAN system prompt injected successfully")
		}
	}

	// Set defaults
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}
	if req.MaxTokens == 0 {
		if req.Type == model.ATS_SCAN {
			req.MaxTokens = 2000 // Higher default for ATS scoring
		} else {
			req.MaxTokens = 1000
		}
	}

	// Validate provider and API key if specified
	if req.Provider != "" {
		if req.APIKey == "" {
			return int(enum.ERROR), "API key is required when provider is specified", nil
		}
		log.Printf("Using provider: %s with model: %s", req.Provider, req.Model)

		// Initialize LLM service if not already done
		if g.LLMService == nil {
			g.LLMService = service.NewLLMService()
		}

		// Make actual API call to LLM provider
		response, err := g.LLMService.SendChatCompletion(req)
		if err != nil {
			log.Printf("Error calling LLM service: %v", err)
			return int(enum.ERROR), fmt.Sprintf("LLM service error: %v", err), err
		}

		log.Println("Chat completion request processed successfully via LLM service")
		return int(enum.DATA_FETCHED), response, nil
	}

	// TODO: Integrate with actual LLM service based on provider
	// For now, return a mock response if no provider specified
	response := model.ChatCompletionResponse{
		ID:    uuid.New().String(),
		Model: req.Model,
		Message: model.ChatMessage{
			Role:    "assistant",
			Content: "This is a placeholder response. Please provide provider and apiKey to use actual LLM service.",
		},
		Usage: model.UsageInfo{
			PromptTokens:     calculateTokens(req.Messages),
			CompletionTokens: 20,
			TotalTokens:      calculateTokens(req.Messages) + 20,
		},
		CreatedAt: time.Now(),
	}

	log.Println("Chat completion request processed with mock response")
	return int(enum.DATA_FETCHED), response, nil
}

// ChatCompletionWithPrompt handles chat completion with a predefined system prompt
func (g *GoLLMController) ChatCompletionWithPrompt(w http.ResponseWriter, r *http.Request, promptType model.ATSPromptType) (int, interface{}, error) {
	var req model.ChatCompletionRequest

	decodeErr := json.NewDecoder(r.Body).Decode(&req)
	if decodeErr != nil {
		log.Println("Error decoding chat completion request:", decodeErr)
		return int(enum.ERROR), "Invalid request body", decodeErr
	}

	// Get the system prompt template
	systemPromptTemplate := model.GetATSPrompt(promptType)
	if systemPromptTemplate == "" {
		log.Println("Invalid prompt type:", promptType)
		return int(enum.ERROR), "Invalid prompt type", nil
	}

	// Inject system prompt as first message if not already present
	if len(req.Messages) == 0 || req.Messages[0].Role != "system" {
		systemMessage := model.ChatMessage{
			Role:    "system",
			Content: systemPromptTemplate,
		}
		req.Messages = append([]model.ChatMessage{systemMessage}, req.Messages...)
	} else {
		// Replace existing system message with the template
		req.Messages[0].Content = systemPromptTemplate
	}

	// Validate request
	if req.Model == "" {
		return int(enum.ERROR), "Model is required", nil
	}

	// Set defaults
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 2000 // Higher default for ATS prompts
	}

	// Validate provider and API key if specified
	if req.Provider != "" {
		if req.APIKey == "" {
			return int(enum.ERROR), "API key is required when provider is specified", nil
		}
		log.Printf("Using provider: %s with model: %s", req.Provider, req.Model)
	}

	// TODO: Integrate with actual LLM service
	// For now, return a mock response
	response := model.ChatCompletionResponse{
		ID:    uuid.New().String(),
		Model: req.Model,
		Message: model.ChatMessage{
			Role:    "assistant",
			Content: "This is a placeholder response with system prompt. Integrate with your LLM service here.",
		},
		Usage: model.UsageInfo{
			PromptTokens:     calculateTokens(req.Messages),
			CompletionTokens: 50,
			TotalTokens:      calculateTokens(req.Messages) + 50,
		},
		CreatedAt: time.Now(),
	}

	log.Printf("Chat completion with prompt type '%s' processed successfully", promptType)
	return int(enum.DATA_FETCHED), response, nil
}

// TextCompletion handles text-based LLM completion requests
func (g *GoLLMController) TextCompletion(w http.ResponseWriter, r *http.Request) (int, interface{}, error) {
	var req model.TextCompletionRequest

	decodeErr := json.NewDecoder(r.Body).Decode(&req)
	if decodeErr != nil {
		log.Println("Error decoding text completion request:", decodeErr)
		return int(enum.ERROR), "Invalid request body", decodeErr
	}

	// Validate request
	if req.Model == "" {
		return int(enum.ERROR), "Model is required", nil
	}
	if req.Prompt == "" {
		return int(enum.ERROR), "Prompt is required", nil
	}

	// Set defaults
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 1000
	}

	// TODO: Integrate with actual LLM service
	// For now, return a mock response
	response := model.TextCompletionResponse{
		ID:    uuid.New().String(),
		Model: req.Model,
		Text:  "This is a placeholder completion. Integrate with your LLM service here.",
		Usage: model.UsageInfo{
			PromptTokens:     len(req.Prompt) / 4, // Rough estimate
			CompletionTokens: 15,
			TotalTokens:      (len(req.Prompt) / 4) + 15,
		},
		CreatedAt: time.Now(),
	}

	log.Println("Text completion request processed successfully")
	return int(enum.DATA_FETCHED), response, nil
}

// calculateTokens is a helper function to estimate token count from messages
func calculateTokens(messages []model.ChatMessage) int {
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
	}
	// Rough estimate: 1 token ≈ 4 characters
	return totalChars / 4
}

// ScanResumeATS performs ATS scoring by scanning resume against job description
// Supports both old format (with provider/apiKey) and new format (with api_id)
func (g *GoLLMController) ScanResumeATS(w http.ResponseWriter, r *http.Request) (int, interface{}, error) {
	var req model.ATSScanRequest

	decodeErr := json.NewDecoder(r.Body).Decode(&req)
	if decodeErr != nil {
		log.Println("Error decoding ATS scan request:", decodeErr)
		return int(enum.ERROR), "Invalid request body", decodeErr
	}

	// Validate request
	if req.APIID == "" {
		return int(enum.ERROR), "api_id is required", nil
	}
	if req.ResumeText == "" {
		return int(enum.ERROR), "resume_text is required", nil
	}
	if req.JobDescription == "" {
		return int(enum.ERROR), "job_description is required", nil
	}

	// Fetch API configuration
	configController := GetControllerInstance(enum.LLMAPIConfigController, enum.MONGODB)
	llmConfigController := configController.(*LLMAPIConfigController)

	apiConfig, err := llmConfigController.GetDecryptedAPIKey(req.APIID)
	if err != nil {
		log.Printf("Error fetching API config: %v", err)
		return int(enum.ERROR), "Failed to fetch API configuration", err
	}
	if apiConfig == nil {
		return int(enum.ERROR), "API configuration not found", nil
	}
	if !apiConfig.IsActive {
		return int(enum.ERROR), "API configuration is not active", nil
	}

	// Set defaults
	temperature := req.Temperature
	if temperature == 0 {
		temperature = 0.7
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2000
	}

	// Build chat completion request
	chatReq := model.ChatCompletionRequest{
		Type:        model.ATS_SCAN,
		Provider:    apiConfig.Provider,
		APIKey:      apiConfig.APIKey,
		Model:       apiConfig.Model,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Messages:    []model.ChatMessage{}, // Will be populated by service
	}

	// Initialize LLM service if not already done
	if g.LLMService == nil {
		g.LLMService = service.NewLLMService()
	}

	// Perform ATS scanning
	log.Printf("Performing ATS scan with config: %s (provider: %s, model: %s)", apiConfig.Name, apiConfig.Provider, apiConfig.Model)
	response, err := g.LLMService.ScanResumeWithJobDescription(chatReq, req.ResumeText, req.JobDescription)
	if err != nil {
		log.Printf("Error performing ATS scan: %v", err)
		return int(enum.ERROR), fmt.Sprintf("ATS scan error: %v", err), err
	}

	// Try to parse the response as ATS score JSON
	atsScore, parseErr := g.LLMService.ParseATSScoreResponse(response.Message.Content)
	if parseErr != nil {
		log.Printf("Warning: Could not parse ATS score JSON: %v", parseErr)
		// Return raw response if parsing fails
		return int(enum.DATA_FETCHED), response, nil
	}

	// Return parsed ATS score
	log.Println("ATS scan completed successfully")
	return int(enum.DATA_FETCHED), map[string]interface{}{
		"id":         response.ID,
		"model":      response.Model,
		"api_config": apiConfig.Name,
		"ats_score":  atsScore,
		"usage":      response.Usage,
		"createdAt":  response.CreatedAt,
	}, nil
}

// TestConnection tests the LLM API connection with provided credentials
func (g *GoLLMController) TestConnection(w http.ResponseWriter, r *http.Request) (int, interface{}, error) {
	var req model.LLMTestConnectionRequest

	decodeErr := json.NewDecoder(r.Body).Decode(&req)
	if decodeErr != nil {
		log.Println("Error decoding test connection request:", decodeErr)
		return int(enum.ERROR), "Invalid request body", decodeErr
	}

	// Validate request
	if req.Provider == "" {
		return int(enum.ERROR), "Provider is required", nil
	}
	if req.Model == "" {
		return int(enum.ERROR), "Model is required", nil
	}
	if req.APIKey == "" {
		return int(enum.ERROR), "API key is required", nil
	}

	// Initialize LLM service if not already done
	if g.LLMService == nil {
		g.LLMService = service.NewLLMService()
	}

	// Build a simple test request
	testReq := model.ChatCompletionRequest{
		Provider:    req.Provider,
		APIKey:      req.APIKey,
		Model:       req.Model,
		Temperature: 0.7,
		MaxTokens:   50,
		Messages: []model.ChatMessage{
			{
				Role:    "user",
				Content: "Hello, this is a test message. Please respond with 'OK'.",
			},
		},
	}

	// Try to make the API call
	log.Printf("Testing connection to %s with model %s", req.Provider, req.Model)
	response, err := g.LLMService.SendChatCompletion(testReq)
	if err != nil {
		log.Printf("Connection test failed: %v", err)
		return int(enum.ERROR), map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Connection failed: %v", err),
			"error":   err.Error(),
		}, nil
	}

	// Connection successful
	log.Printf("Connection test successful for %s/%s", req.Provider, req.Model)
	return int(enum.DATA_FETCHED), map[string]interface{}{
		"success": true,
		"message": "Connection successful! Your credentials are valid.",
		"model":   response.Model,
		"usage":   response.Usage,
	}, nil
}

// FetchOpenRouterModels fetches available models from OpenRouter API
func (g *GoLLMController) FetchOpenRouterModels(w http.ResponseWriter, r *http.Request) (int, interface{}, error) {
	var req model.FetchModelsRequest

	decodeErr := json.NewDecoder(r.Body).Decode(&req)
	if decodeErr != nil {
		log.Println("Error decoding fetch models request:", decodeErr)
		return int(enum.ERROR), "Invalid request body", decodeErr
	}

	// Validate request
	if req.Provider != "openrouter" {
		return int(enum.ERROR), "Only OpenRouter provider is supported for model fetching", nil
	}
	if req.APIKey == "" {
		return int(enum.ERROR), "API key is required", nil
	}

	// Initialize LLM service if not already done
	if g.LLMService == nil {
		g.LLMService = service.NewLLMService()
	}

	// Fetch models from OpenRouter
	log.Printf("Fetching models from OpenRouter")
	models, err := g.LLMService.FetchOpenRouterModels(req.APIKey)
	if err != nil {
		log.Printf("Error fetching OpenRouter models: %v", err)
		return int(enum.ERROR), fmt.Sprintf("Failed to fetch models: %v", err), err
	}

	log.Printf("Successfully fetched %d models from OpenRouter", len(models))
	return int(enum.DATA_FETCHED), map[string]interface{}{
		"provider": "openrouter",
		"models":   models,
		"count":    len(models),
	}, nil
}
