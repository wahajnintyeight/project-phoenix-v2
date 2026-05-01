package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/internal/notifier"
	"project-phoenix/v2/pkg/helper"
	"project-phoenix/v2/pkg/service/worker-service/handlers/validators"

	"go-micro.dev/v4/broker"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type VerifierHandler struct {
	apiKeyController *controllers.APIKeyController
	validatorFactory *validators.ValidatorFactory
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
		validatorFactory: validators.NewValidatorFactory(debugMode),
		processedCount:   0,
		validCount:       0,
		invalidCount:     0,
		debugMode:        debugMode,
		discordNotifier:  discordNotifier,
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
		// Send Discord notification for valid keys with credits
		h.sendValidKeyNotification(key, status, credits, correlationID)
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

// sendValidKeyNotification sends Discord notification for valid keys (DRY helper)
// Only sends if:
// 1. Discord notifier is configured
// 2. Key has not been notified before (NotifiedAt is nil)
// 3. Status is Valid (with credits)
func (h *VerifierHandler) sendValidKeyNotification(key *model.APIKey, status string, credits map[string]interface{}, correlationID string) {
	ctx := helper.LogContext{
		ServiceName:   "worker-service",
		Operation:     "VerifierHandler.sendValidKeyNotification",
		CorrelationID: correlationID,
	}

	// Skip if no Discord notifier configured
	if h.discordNotifier == nil {
		return
	}

	// Skip if already notified
	if key.NotifiedAt != nil {
		helper.LogInfo(ctx, "Skipping notification for key %s (already notified)", key.ID.Hex())
		return
	}

	// Get web scraper URL from environment
	webScraperURL := os.Getenv("PHOENIX_WEB_SCRAPER")
	if webScraperURL == "" {
		webScraperURL = "https://v0-phoenix-scraper.vercel.app/" // Default fallback
	}

	// Send notification
	stats := h.GetStats()
	if err := h.discordNotifier.SendAPIKeyValidation(key.Provider, status, credits, stats, webScraperURL); err != nil {
		helper.LogError(ctx, "Failed to send Discord notification", err)
		return
	}

	// Mark as notified
	if err := h.apiKeyController.UpdateNotifiedAt(key.ID); err != nil {
		helper.LogError(ctx, "Failed to update notified_at timestamp", err)
	} else {
		helper.LogInfo(ctx, "Discord notification sent for key %s", key.ID.Hex())
	}
}

// ValidateKey routes to provider-specific validators
func (h *VerifierHandler) ValidateKey(key *model.APIKey, correlationID string) (string, error) {
	status, _, err := h.ValidateKeyWithCredits(key, correlationID)
	return status, err
}

// ValidateKeyWithCredits routes to provider-specific validators and returns credits info
func (h *VerifierHandler) ValidateKeyWithCredits(key *model.APIKey, correlationID string) (string, map[string]interface{}, error) {
	return h.validatorFactory.ValidateKey(key.Provider, key.KeyValue, correlationID)
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
				// Send Discord notification for valid keys with credits
				if status == model.StatusValid {
					h.sendValidKeyNotification(k, status, credits, correlationID)
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

	// Type assert broker to go-micro Broker interface
	if b, ok := brokerObj.(broker.Broker); ok {
		msg := &broker.Message{
			Body: data,
		}

		helper.LogInfo(ctx, "Publishing message to RabbitMQ topic: keys.validated")
		if err := b.Publish("keys.validated", msg); err != nil {
			helper.LogError(ctx, "Failed to publish message to RabbitMQ, continuing operation", err)
			return nil // Log error but continue operation
		}
		helper.LogInfo(ctx, "Published key validated event for key: %s with status: %s", key.ID.Hex(), status)
	} else {
		helper.LogInfo(ctx, "Broker type assertion failed, skipping publish")
		return nil // Continue operation
	}

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

	helper.LogInfo(ctx, "Starting re-validation cycle for all keys")

	// Retrieve all keys regardless of status
	helper.LogInfo(ctx, "Retrieving all keys from MongoDB")
	allKeys, err := h.apiKeyController.FindAll()
	if err != nil {
		helper.LogError(ctx, "Failed to retrieve keys from MongoDB", err)
		return fmt.Errorf("failed to retrieve keys: %v", err)
	}

	if len(allKeys) == 0 {
		helper.LogInfo(ctx, "No keys to re-validate")
		return nil
	}

	helper.LogInfo(ctx, "Found %d keys to re-validate", len(allKeys))

	// Re-validate keys concurrently
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 3) // Limit to 3 concurrent re-validations (lower than initial validation)

	revalidatedCount := 0
	statusChangedCount := 0

	for _, key := range allKeys {
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

				// Send Discord notification if key became valid (with credits)
				if newStatus == model.StatusValid {
					h.sendValidKeyNotification(k, newStatus, credits, correlationID)
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

	fmt.Printf("Re-validation cycle complete. Re-validated: %d, Status changed: %d",
		revalidatedCount, statusChangedCount)

	return nil
}
