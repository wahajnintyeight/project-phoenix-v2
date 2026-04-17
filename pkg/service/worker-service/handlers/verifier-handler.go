package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/internal/notifier"
	"project-phoenix/v2/pkg/helper"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type VerifierHandler struct {
	apiKeyController *controllers.APIKeyController
	httpClient       *http.Client
	processedCount   int
	validCount       int
	invalidCount     int
	debugMode        bool
	discordNotifier  *notifier.DiscordNotifier
}

// NewVerifierHandler creates a new VerifierHandler instance
func NewVerifierHandler() *VerifierHandler {
	// Get APIKeyController from the controller factory
	apiKeyController := controllers.GetControllerInstance(enum.APIKeyController, enum.MONGODB).(*controllers.APIKeyController)

	// Check if debug mode is enabled via environment variable
	debugMode := os.Getenv("VERIFIER_DEBUG") == "true"

	// Initialize Discord notifier if webhook URL is provided
	var discordNotifier *notifier.DiscordNotifier
	if webhookURL := os.Getenv("DISCORD_WEBHOOK_VERIFIER"); webhookURL != "" {
		discordNotifier = notifier.NewDiscordNotifier(webhookURL)
		log.Println("Discord notifications enabled for verifier")
	}

	return &VerifierHandler{
		apiKeyController: apiKeyController,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		processedCount:  0,
		validCount:      0,
		invalidCount:    0,
		debugMode:       debugMode,
		discordNotifier: discordNotifier,
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
	status, credits, err := h.ValidateKeyWithCredits(key, correlationID)
	if err != nil {
		helper.LogError(ctx, "Validation error for key %s", err, keyID.Hex())
		// Update status to Error
		if updateErr := h.apiKeyController.UpdateStatus(keyID, model.StatusError); updateErr != nil {
			helper.LogError(ctx, "Failed to update error status in MongoDB", updateErr)
		}
		h.processedCount++
		return err
	}

	// Update key status and credits in database
	helper.LogInfo(ctx, "Updating key status and credits in MongoDB: %s", status)
	if err := h.apiKeyController.UpdateStatusAndCredits(keyID, status, credits); err != nil {
		helper.LogError(ctx, "Failed to update key status in MongoDB", err)
		return fmt.Errorf("failed to update key status: %v", err)
	}

	// Update counters
	h.processedCount++
	if status == model.StatusValid {
		h.validCount++

		// Send Discord notification ONLY for valid keys with credits (not for invalid or no credits)
		if h.discordNotifier != nil && key.NotifiedAt == nil {
			stats := h.GetStats()
			if err := h.discordNotifier.SendAPIKeyValidation(key.Provider, status, credits, stats); err != nil {
				helper.LogError(ctx, "Failed to send Discord notification", err)
			} else {
				// Mark as notified
				if err := h.apiKeyController.UpdateNotifiedAt(keyID); err != nil {
					helper.LogError(ctx, "Failed to update notified_at timestamp", err)
				}
			}
		}
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
	status, _, err := h.ValidateKeyWithCredits(key, correlationID)
	return status, err
}

// ValidateKeyWithCredits routes to provider-specific validators and returns credits info
func (h *VerifierHandler) ValidateKeyWithCredits(key *model.APIKey, correlationID string) (string, map[string]interface{}, error) {
	// log.Println("Verifying ", key.Provider)
	switch key.Provider {
	case model.ProviderOpenAI:
		status, err := h.ValidateOpenAIKey(key.KeyValue, correlationID)
		return status, nil, err
	case model.ProviderAnthropic:
		status, err := h.ValidateAnthropicKey(key.KeyValue, correlationID)
		return status, nil, err
	case model.ProviderGoogle:
		status, err := h.ValidateGoogleKey(key.KeyValue, correlationID)
		return status, nil, err
	case model.ProviderOpenRouter:
		return h.ValidateOpenRouterKeyWithCredits(key.KeyValue, correlationID)
	default:
		return model.StatusError, nil, fmt.Errorf("unknown provider: %s", key.Provider)
	}
}

// ValidateOpenAIKey validates an OpenAI API key
func (h *VerifierHandler) ValidateOpenAIKey(keyValue string, correlationID string) (string, error) {
	url := "https://api.openai.com/v1/chat/completions"

	// Using gpt-4o-mini for verification - still available in API as of April 2026
	// This tests actual inference capability, not just key existence
	requestBody := map[string]interface{}{
		"model":      "gpt-5.4",
		"max_tokens": 1,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
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

	req.Header.Set("Authorization", "Bearer "+keyValue)
	req.Header.Set("Content-Type", "application/json")

	status, err := h.executeRequestWithRetry(req, correlationID)
	return status, err
}

// ValidateAnthropicKey validates an Anthropic API key
func (h *VerifierHandler) ValidateAnthropicKey(keyValue string, correlationID string) (string, error) {
	url := "https://api.anthropic.com/v1/messages"

	// Minimal request body to test authentication
	// Using claude-haiku-4-5 as claude-3-haiku is being retired in April 2026
	requestBody := map[string]interface{}{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 1024,
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

	req.Header.Set("X-Api-Key", keyValue)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	status, err := h.executeRequestWithRetry(req, correlationID)
	return status, err
}

// ValidateGoogleKey validates a Google AI API key
func (h *VerifierHandler) ValidateGoogleKey(keyValue string, correlationID string) (string, error) {
	// Use the models list endpoint — lightweight GET, no token consumption
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", keyValue)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return model.StatusError, err
	}

	status, err := h.executeRequestWithRetry(req, correlationID)
	return status, err
}

// ValidateOpenRouterKey validates an OpenRouter API key using the credits endpoint
func (h *VerifierHandler) ValidateOpenRouterKey(keyValue string, correlationID string) (string, error) {
	status, _, err := h.ValidateOpenRouterKeyWithCredits(keyValue, correlationID)
	return status, err
}

// ValidateOpenRouterKeyWithCredits validates an OpenRouter API key and returns credits info
func (h *VerifierHandler) ValidateOpenRouterKeyWithCredits(keyValue string, correlationID string) (string, map[string]interface{}, error) {
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "VerifierHandler.ValidateOpenRouterKeyWithCredits",
		CorrelationID: correlationID,
	}

	// Use the /credits endpoint to check key validity and get credit balance
	url := "https://openrouter.ai/api/v1/credits"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return model.StatusError, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+keyValue)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		helper.LogError(ctx, "HTTP request error", err)
		return model.StatusError, nil, err
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		helper.LogError(ctx, "Failed to read response body", err)
		return model.StatusError, nil, err
	}

	// Log response if debug mode is enabled
	if h.debugMode {
		log.Printf("[DEBUG] OpenRouter Credits API Response:\n")
		log.Printf("  URL: %s\n", url)
		log.Printf("  Status Code: %d\n", resp.StatusCode)
		log.Printf("  Body: %s\n", string(bodyBytes))
	}

	// Check status code
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return model.StatusInvalid, nil, nil
	}

	if resp.StatusCode != 200 {
		return model.StatusError, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse credits response
	var creditsResp struct {
		Data struct {
			TotalCredits *float64 `json:"total_credits"`
			TotalUsage   *float64 `json:"total_usage"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &creditsResp); err != nil {
		helper.LogError(ctx, "Failed to parse credits response", err)
		return model.StatusError, nil, err
	}

	// Validate that we received credit information
	if creditsResp.Data.TotalCredits == nil {
		helper.LogError(ctx, "OpenRouter API returned 200 but missing total_credits field", nil)
		return model.StatusError, nil, fmt.Errorf("missing total_credits in response")
	}

	totalCredits := *creditsResp.Data.TotalCredits
	totalUsage := float64(0)
	if creditsResp.Data.TotalUsage != nil {
		totalUsage = *creditsResp.Data.TotalUsage
	}

	// Store credits info
	credits := map[string]interface{}{
		"total_credits": totalCredits,
		"total_usage":   totalUsage,
		"checked_at":    time.Now(),
	}

	// Determine status based on credits
	status := model.StatusValid
	if totalCredits <= 0 {
		status = model.StatusValidNoCredits
		helper.LogInfo(ctx, "OpenRouter key has no credits remaining")
	}

	helper.LogInfo(ctx, "OpenRouter key validated: credits=%.2f, usage=%.2f, status=%s",
		totalCredits, totalUsage, status)

	return status, credits, nil
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

		// Read response body for debugging
		var bodyString string
		if resp.Body != nil && h.debugMode {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err == nil {
				bodyString = string(bodyBytes)
			}
		}

		// Log full response if debug mode is enabled
		if h.debugMode {
			log.Printf("[DEBUG] Provider API Response:\n")
			log.Printf("  URL: %s\n", req.URL.String())
			log.Printf("  Status Code: %d\n", resp.StatusCode)
			log.Printf("  Status: %s\n", resp.Status)
			log.Printf("  Headers: %+v\n", resp.Header)
			if len(bodyString) > 0 && len(bodyString) < 10000 {
				log.Printf("  Body: %s\n", bodyString)
			} else if len(bodyString) >= 10000 {
				log.Printf("  Body: [truncated - %d bytes]\n", len(bodyString))
			} else {
				log.Printf("  Body: [empty or not read]\n")
			}
		}

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
			status, credits, err := h.ValidateKeyWithCredits(k, correlationID)
			if err != nil {
				helper.LogError(keyCtx, "Validation error for key %s", err, k.ID.Hex())
				status = model.StatusError
			}

			// Update key status and credits
			helper.LogInfo(keyCtx, "Updating key status and credits in MongoDB: %s", status)
			if updateErr := h.apiKeyController.UpdateStatusAndCredits(k.ID, status, credits); updateErr != nil {
				helper.LogError(keyCtx, "Failed to update key status in MongoDB", updateErr)
				return
			}

			// Update counters
			h.processedCount++
			if status == model.StatusValid || status == model.StatusValidNoCredits {
				h.validCount++

				// Send Discord notification ONLY for valid keys with credits (not for invalid or no credits)
				if h.discordNotifier != nil && status == model.StatusValid && k.NotifiedAt == nil {
					stats := h.GetStats()
					if err := h.discordNotifier.SendAPIKeyValidation(k.Provider, status, credits, stats); err != nil {
						helper.LogError(keyCtx, "Failed to send Discord notification", err)
					} else {
						// Mark as notified
						if err := h.apiKeyController.UpdateNotifiedAt(k.ID); err != nil {
							helper.LogError(keyCtx, "Failed to update notified_at timestamp", err)
						}
					}
				}
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

// RunRevalidationCycle retrieves valid keys and re-validates them concurrently
func (h *VerifierHandler) RunRevalidationCycle(broker interface{}) error {
	correlationID := helper.GenerateCorrelationID()
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "VerifierHandler.RunRevalidationCycle",
		CorrelationID: correlationID,
	}

	helper.LogInfo(ctx, "Starting re-validation cycle for valid keys")

	// Retrieve all valid keys (including ValidNoCredits)
	helper.LogInfo(ctx, "Retrieving valid keys from MongoDB")
	validKeys, err := h.apiKeyController.FindByStatus(model.StatusValid)
	if err != nil {
		helper.LogError(ctx, "Failed to retrieve valid keys from MongoDB", err)
		return fmt.Errorf("failed to retrieve valid keys: %v", err)
	}

	validNoCreditsKeys, err := h.apiKeyController.FindByStatus(model.StatusValidNoCredits)
	if err != nil {
		helper.LogError(ctx, "Failed to retrieve ValidNoCredits keys from MongoDB", err)
	} else {
		validKeys = append(validKeys, validNoCreditsKeys...)
	}

	if len(validKeys) == 0 {
		helper.LogInfo(ctx, "No valid keys to re-validate")
		return nil
	}

	helper.LogInfo(ctx, "Found %d valid keys to re-validate", len(validKeys))

	// Re-validate keys concurrently
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 3) // Limit to 3 concurrent re-validations (lower than initial validation)

	revalidatedCount := 0
	statusChangedCount := 0

	for _, key := range validKeys {
		wg.Add(1)
		go func(k *model.APIKey) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			keyCtx := helper.LogContext{
				ServiceName:   "worker-service",
				Operation:     "RevalidateKey",
				CorrelationID: correlationID,
			}

			// Re-validate the key
			helper.LogInfo(keyCtx, "Re-validating key %s for provider: %s", k.ID.Hex(), k.Provider)
			newStatus, credits, err := h.ValidateKeyWithCredits(k, correlationID)
			if err != nil {
				helper.LogError(keyCtx, "Re-validation error for key %s", err, k.ID.Hex())
				// Don't change status on error during re-validation
				return
			}

			revalidatedCount++

			// Only update if status changed
			if newStatus != k.Status {
				statusChangedCount++
				helper.LogInfo(keyCtx, "Status changed for key %s: %s -> %s", k.ID.Hex(), k.Status, newStatus)

				// Update key status and credits
				if updateErr := h.apiKeyController.UpdateStatusAndCredits(k.ID, newStatus, credits); updateErr != nil {
					helper.LogError(keyCtx, "Failed to update key status in MongoDB", updateErr)
					return
				}

				// Send Discord notification ONLY if:
				// 1. New status is Valid (with credits, not ValidNoCredits)
				// 2. Key was not already notified (NotifiedAt is nil)
				// This prevents duplicate notifications during re-validation
				if h.discordNotifier != nil && newStatus == model.StatusValid && k.NotifiedAt == nil {
					stats := h.GetStats()
					if err := h.discordNotifier.SendAPIKeyValidation(k.Provider, newStatus, credits, stats); err != nil {
						helper.LogError(keyCtx, "Failed to send Discord notification", err)
					} else {
						// Mark as notified
						if err := h.apiKeyController.UpdateNotifiedAt(k.ID); err != nil {
							helper.LogError(keyCtx, "Failed to update notified_at timestamp", err)
						}
					}
				}

				// Publish status change event
				if broker != nil {
					helper.LogInfo(keyCtx, "Publishing status change event to RabbitMQ")
					if err := h.PublishKeyValidated(k, newStatus, broker, correlationID); err != nil {
						helper.LogError(keyCtx, "Failed to publish status change event to RabbitMQ", err)
					}
				}
			} else {
				// Status unchanged, but update credits if available
				if credits != nil && len(credits) > 0 {
					if updateErr := h.apiKeyController.UpdateStatusAndCredits(k.ID, newStatus, credits); updateErr != nil {
						helper.LogError(keyCtx, "Failed to update credits in MongoDB", updateErr)
					}
				}
				helper.LogInfo(keyCtx, "Key %s status unchanged: %s", k.ID.Hex(), newStatus)
			}
		}(key)
	}

	wg.Wait()

	helper.LogInfo(ctx, "Re-validation cycle complete. Re-validated: %d, Status changed: %d",
		revalidatedCount, statusChangedCount)

	return nil
}
