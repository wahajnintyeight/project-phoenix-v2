package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/pkg/helper"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type VerifierHandler struct {
	apiKeyController *controllers.APIKeyController
	httpClient       *http.Client
	processedCount   int
	validCount       int
	invalidCount     int
}

// NewVerifierHandler creates a new VerifierHandler instance
func NewVerifierHandler() *VerifierHandler {
	// Get APIKeyController from the controller factory
	apiKeyController := controllers.GetControllerInstance(enum.APIKeyController, enum.MONGODB).(*controllers.APIKeyController)

	return &VerifierHandler{
		apiKeyController: apiKeyController,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		processedCount: 0,
		validCount:     0,
		invalidCount:   0,
	}
}

// Process handles RabbitMQ messages from keys.discovered topic
func (h *VerifierHandler) Process(data map[string]interface{}) error {
	correlationID := helper.GenerateCorrelationID()
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "VerifierHandler.Process",
		CorrelationID: correlationID,
	}

	helper.LogInfo(ctx, "Processing key validation task from RabbitMQ")

	// Extract key ID from message
	keyIDStr, ok := data["id"].(string)
	if !ok {
		helper.LogError(ctx, "Missing or invalid key ID in RabbitMQ message", fmt.Errorf("invalid message format"))
		return fmt.Errorf("missing or invalid key ID in message")
	}

	keyID, err := primitive.ObjectIDFromHex(keyIDStr)
	if err != nil {
		helper.LogError(ctx, "Invalid key ID format", err)
		return fmt.Errorf("invalid key ID format: %v", err)
	}

	// Retrieve the key from database
	helper.LogInfo(ctx, "Retrieving key from MongoDB: %s", keyID.Hex())
	key, err := h.apiKeyController.FindOne(map[string]interface{}{"_id": keyID})
	if err != nil {
		helper.LogError(ctx, "Failed to retrieve key from MongoDB", err)
		return fmt.Errorf("failed to retrieve key: %v", err)
	}

	// Validate the key
	helper.LogInfo(ctx, "Validating key against provider API: %s", key.Provider)
	status, err := h.ValidateKey(key, correlationID)
	if err != nil {
		helper.LogError(ctx, "Validation error for key %s", err, keyID.Hex())
		// Update status to Error
		if updateErr := h.apiKeyController.UpdateStatus(keyID, model.StatusError); updateErr != nil {
			helper.LogError(ctx, "Failed to update error status in MongoDB", updateErr)
		}
		h.processedCount++
		return err
	}

	// Update key status in database
	helper.LogInfo(ctx, "Updating key status in MongoDB: %s", status)
	if err := h.apiKeyController.UpdateStatus(keyID, status); err != nil {
		helper.LogError(ctx, "Failed to update key status in MongoDB", err)
		return fmt.Errorf("failed to update key status: %v", err)
	}

	// Update counters
	h.processedCount++
	if status == model.StatusValid || status == model.StatusValidNoCredits {
		h.validCount++
	} else if status == model.StatusInvalid {
		h.invalidCount++
	}

	helper.LogInfo(ctx, "Key %s validated with status: %s", keyID.Hex(), status)

	return nil
}

// GetStats returns handler statistics
func (h *VerifierHandler) GetStats() map[string]int {
	return map[string]int{
		"processed": h.processedCount,
		"valid":     h.validCount,
		"invalid":   h.invalidCount,
	}
}

// ValidateKey routes to provider-specific validators
func (h *VerifierHandler) ValidateKey(key *model.APIKey, correlationID string) (string, error) {
	switch key.Provider {
	case model.ProviderOpenAI:
		return h.ValidateOpenAIKey(key.KeyValue, correlationID)
	case model.ProviderAnthropic:
		return h.ValidateAnthropicKey(key.KeyValue, correlationID)
	case model.ProviderGoogle:
		return h.ValidateGoogleKey(key.KeyValue, correlationID)
	case model.ProviderOpenRouter:
		return h.ValidateOpenRouterKey(key.KeyValue, correlationID)
	default:
		return model.StatusError, fmt.Errorf("unknown provider: %s", key.Provider)
	}
}

// ValidateOpenAIKey validates an OpenAI API key
func (h *VerifierHandler) ValidateOpenAIKey(keyValue string, correlationID string) (string, error) {
	url := "https://api.openai.com/v1/models"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return model.StatusError, err
	}

	req.Header.Set("Authorization", "Bearer "+keyValue)

	status, err := h.executeRequestWithRetry(req, correlationID)
	return status, err
}

// ValidateAnthropicKey validates an Anthropic API key
func (h *VerifierHandler) ValidateAnthropicKey(keyValue string, correlationID string) (string, error) {
	url := "https://api.anthropic.com/v1/messages"

	// Minimal request body to test authentication
	requestBody := map[string]interface{}{
		"model":      "claude-3-haiku-20240307",
		"max_tokens": 1,
		"messages": []map[string]string{
			{"role": "user", "content": "test"},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return model.StatusError, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return model.StatusError, err
	}

	req.Header.Set("x-api-key", keyValue)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	status, err := h.executeRequestWithRetry(req, correlationID)
	return status, err
}

// ValidateGoogleKey validates a Google AI API key
func (h *VerifierHandler) ValidateGoogleKey(keyValue string, correlationID string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", keyValue)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return model.StatusError, err
	}

	status, err := h.executeRequestWithRetry(req, correlationID)
	return status, err
}

// ValidateOpenRouterKey validates an OpenRouter API key
func (h *VerifierHandler) ValidateOpenRouterKey(keyValue string, correlationID string) (string, error) {
	url := "https://openrouter.ai/api/v1/models"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return model.StatusError, err
	}

	req.Header.Set("Authorization", "Bearer "+keyValue)

	status, err := h.executeRequestWithRetry(req, correlationID)
	return status, err
}

// executeRequestWithRetry executes HTTP request with retry logic for 5xx errors
func (h *VerifierHandler) executeRequestWithRetry(req *http.Request, correlationID string) (string, error) {
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "VerifierHandler.executeRequestWithRetry",
		CorrelationID: correlationID,
	}

	maxRetries := 3
	retryDelay := 2 * time.Second

	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			helper.LogInfo(ctx, "Retry attempt %d/%d after %v", attempt, maxRetries, retryDelay)
			time.Sleep(retryDelay)
		}

		resp, lastErr = h.httpClient.Do(req)
		if lastErr != nil {
			helper.LogError(ctx, "HTTP request error", lastErr)
			continue
		}

		defer resp.Body.Close()

		// Determine status from response
		status := h.determineStatusFromResponse(resp)
		helper.LogInfo(ctx, "Provider API response: HTTP %d, determined status: %s", resp.StatusCode, status)

		// Retry on 5xx errors
		if resp.StatusCode >= 500 && resp.StatusCode < 600 && attempt < maxRetries {
			helper.LogInfo(ctx, "Received %d status, retrying...", resp.StatusCode)
			continue
		}

		return status, nil
	}

	// All retries exhausted
	if lastErr != nil {
		helper.LogError(ctx, "Max retries exceeded", lastErr)
		return model.StatusError, lastErr
	}

	helper.LogError(ctx, "Max retries exceeded", fmt.Errorf("all attempts failed"))
	return model.StatusError, fmt.Errorf("max retries exceeded")
}

// determineStatusFromResponse determines key status from HTTP response
func (h *VerifierHandler) determineStatusFromResponse(resp *http.Response) string {
	switch {
	case resp.StatusCode == 200:
		// Check if response indicates no credits (provider-specific logic)
		// For now, assume Valid if 200
		return model.StatusValid
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return model.StatusInvalid
	default:
		return model.StatusError
	}
}

// RunValidationCycle retrieves pending keys and validates them concurrently
func (h *VerifierHandler) RunValidationCycle(broker interface{}) error {
	correlationID := helper.GenerateCorrelationID()
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "VerifierHandler.RunValidationCycle",
		CorrelationID: correlationID,
	}

	helper.LogInfo(ctx, "Starting validation cycle")

	// Retrieve all pending keys
	helper.LogInfo(ctx, "Retrieving pending keys from MongoDB")
	pendingKeys, err := h.apiKeyController.FindPendingKeys()
	if err != nil {
		helper.LogError(ctx, "Failed to retrieve pending keys from MongoDB", err)
		return fmt.Errorf("failed to retrieve pending keys: %v", err)
	}

	if len(pendingKeys) == 0 {
		helper.LogInfo(ctx, "No pending keys to validate")
		return nil
	}

	helper.LogInfo(ctx, "Found %d pending keys to validate", len(pendingKeys))

	// Validate keys concurrently
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Limit to 5 concurrent validations

	for _, key := range pendingKeys {
		wg.Add(1)
		go func(k *model.APIKey) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			keyCtx := helper.LogContext{
				ServiceName:   "worker-service",
				Operation:     "ValidateKey",
				CorrelationID: correlationID,
			}

			// Validate the key
			helper.LogInfo(keyCtx, "Validating key %s against provider: %s", k.ID.Hex(), k.Provider)
			status, err := h.ValidateKey(k, correlationID)
			if err != nil {
				helper.LogError(keyCtx, "Validation error for key %s", err, k.ID.Hex())
				status = model.StatusError
			}

			// Update key status
			helper.LogInfo(keyCtx, "Updating key status in MongoDB: %s", status)
			if updateErr := h.apiKeyController.UpdateStatus(k.ID, status); updateErr != nil {
				helper.LogError(keyCtx, "Failed to update key status in MongoDB", updateErr)
				return
			}

			// Update counters
			h.processedCount++
			if status == model.StatusValid || status == model.StatusValidNoCredits {
				h.validCount++
			} else if status == model.StatusInvalid {
				h.invalidCount++
			}

			// Publish validation result
			if broker != nil {
				helper.LogInfo(keyCtx, "Publishing validation result to RabbitMQ")
				if err := h.PublishKeyValidated(k, status, broker, correlationID); err != nil {
					helper.LogError(keyCtx, "Failed to publish validation result to RabbitMQ", err)
				}
			}

			helper.LogInfo(keyCtx, "Key %s validated with status: %s", k.ID.Hex(), status)
		}(key)
	}

	wg.Wait()

	// Enforce valid key limit after validation
	helper.LogInfo(ctx, "Enforcing valid key limit")
	if err := h.EnforceValidKeyLimit(correlationID); err != nil {
		helper.LogError(ctx, "Failed to enforce valid key limit", err)
	}

	helper.LogInfo(ctx, "Validation cycle complete. Processed: %d, Valid: %d, Invalid: %d",
		h.processedCount, h.validCount, h.invalidCount)

	return nil
}

// PublishKeyValidated publishes a message to RabbitMQ keys.validated topic
func (h *VerifierHandler) PublishKeyValidated(key *model.APIKey, status string, brokerObj interface{}, correlationID string) error {
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "VerifierHandler.PublishKeyValidated",
		CorrelationID: correlationID,
	}

	if brokerObj == nil {
		helper.LogError(ctx, "RabbitMQ broker is unavailable, skipping publish", nil)
		return nil // Continue operation even if broker is unavailable
	}

	payload := map[string]interface{}{
		"id":           key.ID.Hex(),
		"provider":     key.Provider,
		"status":       status,
		"validated_at": time.Now(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		helper.LogError(ctx, "Failed to marshal RabbitMQ payload, continuing operation", err)
		return nil // Continue operation even if marshal fails
	}

	// Type assert broker to the correct type
	if b, ok := brokerObj.(interface {
		Publish(string, interface{}) error
	}); ok {
		msg := map[string]interface{}{
			"Body": data,
		}
		helper.LogInfo(ctx, "Publishing message to RabbitMQ topic: keys.validated")
		if err := b.Publish("keys.validated", msg); err != nil {
			helper.LogError(ctx, "Failed to publish message to RabbitMQ, continuing operation", err)
			return nil // Log error but continue operation
		}
	} else {
		helper.LogInfo(ctx, "Warning: broker does not support Publish method, skipping publish")
		return nil // Continue operation
	}

	helper.LogInfo(ctx, "Published key validated event for key: %s with status: %s", key.ID.Hex(), status)
	return nil
}

// EnforceValidKeyLimit deletes oldest valid keys when count exceeds configured limit
func (h *VerifierHandler) EnforceValidKeyLimit(correlationID string) error {
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "VerifierHandler.EnforceValidKeyLimit",
		CorrelationID: correlationID,
	}

	// Get max valid keys from environment (default: 50)
	maxValidKeys := 50
	if maxKeysStr := os.Getenv("MAX_VALID_KEYS"); maxKeysStr != "" {
		if parsed, err := strconv.Atoi(maxKeysStr); err == nil {
			maxValidKeys = parsed
		}
	}

	// Get count of valid keys
	helper.LogInfo(ctx, "Checking valid key count in MongoDB")
	validKeys, err := h.apiKeyController.FindByStatus(model.StatusValid)
	if err != nil {
		helper.LogError(ctx, "Failed to retrieve valid keys from MongoDB", err)
		return fmt.Errorf("failed to retrieve valid keys: %v", err)
	}

	if len(validKeys) <= maxValidKeys {
		helper.LogInfo(ctx, "Valid key count (%d) is within limit (%d)", len(validKeys), maxValidKeys)
		return nil
	}

	// Delete oldest keys to enforce limit
	helper.LogInfo(ctx, "Deleting oldest valid keys from MongoDB to enforce limit")
	if err := h.apiKeyController.DeleteOldestValidKeys(maxValidKeys); err != nil {
		helper.LogError(ctx, "Failed to delete oldest valid keys from MongoDB", err)
		return fmt.Errorf("failed to delete oldest valid keys: %v", err)
	}

	helper.LogInfo(ctx, "Enforced valid key limit: deleted %d keys", len(validKeys)-maxValidKeys)
	return nil
}
