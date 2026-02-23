package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/pkg/helper"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

type LLMAPIConfigController struct {
	DB db.DBInterface
}

func (l *LLMAPIConfigController) GetCollectionName() string {
	return "llm_api_configs"
}

// CreateAPIConfig creates a new LLM API configuration with encrypted API key
func (l *LLMAPIConfigController) CreateAPIConfig(w http.ResponseWriter, r *http.Request) (int, interface{}, error) {
	var req model.LLMAPIConfigRequest

	decodeErr := json.NewDecoder(r.Body).Decode(&req)
	if decodeErr != nil {
		log.Println("Error decoding API config request:", decodeErr)
		return int(enum.ERROR), "Invalid request body", decodeErr
	}

	// Validate request
	if req.Name == "" {
		return int(enum.ERROR), "Name is required", nil
	}
	if req.Provider == "" {
		return int(enum.ERROR), "Provider is required", nil
	}
	if req.Model == "" {
		return int(enum.ERROR), "Model is required", nil
	}
	if req.APIKey == "" {
		return int(enum.ERROR), "API key is required", nil
	}

	// Validate provider
	validProviders := map[string]bool{
		"openai":     true,
		"anthropic":  true,
		"groq":       true,
		"openrouter": true,
		"ollama":     true,
	}
	if !validProviders[req.Provider] {
		return int(enum.ERROR), "Invalid provider. Must be: openai, anthropic, groq, openrouter, or ollama", nil
	}

	// Encrypt API key
	encryptedKey, encryptErr := helper.EncryptAPIKey(req.APIKey)
	if encryptErr != nil {
		log.Println("Error encrypting API key:", encryptErr)
		return int(enum.ERROR), "Failed to encrypt API key", encryptErr
	}

	// Get user from context (you may need to adjust this based on your auth)
	// For now, using a placeholder
	createdBy := "system" // TODO: Get from authenticated user context

	// Create config
	config := model.LLMAPIConfig{
		Name:            req.Name,
		Provider:        req.Provider,
		Model:           req.Model,
		EncryptedAPIKey: encryptedKey,
		IsActive:        req.IsActive,
		CreatedBy:       createdBy,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Save to database
	result, err := l.DB.Create(config, l.GetCollectionName())
	if err != nil {
		log.Println("Error saving API config:", err)
		return int(enum.ERROR), "Failed to save API configuration", err
	}

	config.ID = helper.InterfaceToString(result["_id"])

	log.Printf("Created LLM API config: %s (ID: %s)", config.Name, config.ID)
	return int(enum.DATA_FETCHED), config.ToResponse(), nil
}

// ListAPIConfigs lists all LLM API configurations (without API keys)
func (l *LLMAPIConfigController) ListAPIConfigs(w http.ResponseWriter, r *http.Request) (int, interface{}, error) {
	// Query all configs
	query := bson.M{}

	// Optional: filter by active status
	activeOnly := r.URL.Query().Get("active")
	if activeOnly == "true" {
		query["isActive"] = true
	}

	// Use pagination (get all by setting high page size)
	_, _, results, err := l.DB.FindAllWithPagination(query, 1000, l.GetCollectionName())
	if err != nil {
		log.Println("Error fetching API configs:", err)
		return int(enum.ERROR), "Failed to fetch API configurations", err
	}

	// Convert to response format (without API keys)
	var configs []model.LLMAPIConfigResponse
	for _, result := range results {
		var config model.LLMAPIConfig
		mapErr := helper.MapToStruct(result, &config)
		if mapErr != nil {
			log.Println("Error converting config:", mapErr)
			continue
		}
		configs = append(configs, config.ToResponse())
	}

	log.Printf("Listed %d LLM API configs", len(configs))
	return int(enum.DATA_FETCHED), configs, nil
}

// GetAPIConfig retrieves a specific API configuration by ID (without API key)
func (l *LLMAPIConfigController) GetAPIConfig(w http.ResponseWriter, r *http.Request) (int, interface{}, error) {
	configID := r.URL.Query().Get("id")
	if configID == "" {
		return int(enum.ERROR), "Config ID is required", nil
	}

	query := bson.M{"_id": configID}
	result, err := l.DB.FindOne(query, l.GetCollectionName())
	if err != nil {
		log.Println("Error fetching API config:", err)
		return int(enum.ERROR), "Failed to fetch API configuration", err
	}

	if result == nil {
		return int(enum.ERROR), "API configuration not found", nil
	}

	var config model.LLMAPIConfig
	mapErr := helper.MapToStruct(result, &config)
	if mapErr != nil {
		log.Println("Error converting config:", mapErr)
		return int(enum.ERROR), "Failed to parse API configuration", mapErr
	}

	log.Printf("Retrieved LLM API config: %s", config.ID)
	return int(enum.DATA_FETCHED), config.ToResponse(), nil
}

// UpdateAPIConfig updates an existing API configuration
func (l *LLMAPIConfigController) UpdateAPIConfig(w http.ResponseWriter, r *http.Request) (int, interface{}, error) {
	configID := r.URL.Query().Get("id")
	if configID == "" {
		return int(enum.ERROR), "Config ID is required", nil
	}

	var req model.LLMAPIConfigRequest
	decodeErr := json.NewDecoder(r.Body).Decode(&req)
	if decodeErr != nil {
		log.Println("Error decoding update request:", decodeErr)
		return int(enum.ERROR), "Invalid request body", decodeErr
	}

	// Build update document
	update := bson.M{
		"updatedAt": time.Now(),
	}

	if req.Name != "" {
		update["name"] = req.Name
	}
	if req.Provider != "" {
		update["provider"] = req.Provider
	}
	if req.Model != "" {
		update["model"] = req.Model
	}
	if req.APIKey != "" {
		// Encrypt new API key
		encryptedKey, encryptErr := helper.EncryptAPIKey(req.APIKey)
		if encryptErr != nil {
			log.Println("Error encrypting API key:", encryptErr)
			return int(enum.ERROR), "Failed to encrypt API key", encryptErr
		}
		update["encryptedApiKey"] = encryptedKey
	}
	update["isActive"] = req.IsActive

	// Update in database
	query := bson.M{"_id": configID}
	_, err := l.DB.Update(query, bson.M{"$set": update}, l.GetCollectionName())
	if err != nil {
		log.Println("Error updating API config:", err)
		return int(enum.ERROR), "Failed to update API configuration", err
	}

	log.Printf("Updated LLM API config: %s", configID)
	return int(enum.DATA_FETCHED), "API configuration updated successfully", nil
}

// DeleteAPIConfig deletes an API configuration
func (l *LLMAPIConfigController) DeleteAPIConfig(w http.ResponseWriter, r *http.Request) (int, error) {
	configID := r.URL.Query().Get("id")
	if configID == "" {
		return int(enum.ERROR), nil
	}

	query := bson.M{"_id": configID}
	_, err := l.DB.Delete(query, l.GetCollectionName())
	if err != nil {
		log.Println("Error deleting API config:", err)
		return int(enum.ERROR), err
	}

	log.Printf("Deleted LLM API config: %s", configID)
	return int(enum.DATA_FETCHED), nil
}

// GetDecryptedAPIKey retrieves and decrypts the API key for internal use
func (l *LLMAPIConfigController) GetDecryptedAPIKey(configID string) (*model.LLMAPIConfig, error) {
	query := bson.M{"_id": configID}
	result, err := l.DB.FindOne(query, l.GetCollectionName())
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	var config model.LLMAPIConfig
	mapErr := helper.MapToStruct(result, &config)
	if mapErr != nil {
		return nil, mapErr
	}

	// Decrypt API key
	decryptedKey, decryptErr := helper.DecryptAPIKey(config.EncryptedAPIKey)
	if decryptErr != nil {
		log.Println("Error decrypting API key:", decryptErr)
		return nil, decryptErr
	}

	config.APIKey = decryptedKey
	return &config, nil
}
