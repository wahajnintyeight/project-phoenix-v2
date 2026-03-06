package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/internal/service"
	"project-phoenix/v2/internal/vectorstore"

	"github.com/google/uuid"
)


type ScreenshotHandler struct {
	processedCount int
	failedCount    int
	llmService     *service.LLMService
	apiKey         string
	modelName      string
	qdrantClient   *vectorstore.QdrantClient
}

func NewScreenshotHandler() *ScreenshotHandler {
	// Initialize Qdrant client
	qdrantClient, err := vectorstore.NewQdrantClient()
	if err != nil {
		log.Printf("⚠️  Warning: Failed to initialize Qdrant client: %v", err)
		log.Println("Continuing without vector storage...")
	} else {
		log.Printf("✓ Qdrant client connected successfully")
	}

	return &ScreenshotHandler{
		processedCount: 0,
		failedCount:    0,
		llmService:     service.NewLLMService(),
		apiKey:         os.Getenv("OPENROUTER_API_KEY"),
		modelName:      getEnvOrDefault("OPENROUTER_MODEL", "anthropic/claude-3.5-sonnet"),
		qdrantClient:   qdrantClient,
	}
}

// Process handles screenshot processing tasks
func (h *ScreenshotHandler) Process(data map[string]interface{}) error {
	log.Println("Processing screenshot task:", data)

	// Extract screenshot metadata
	metadata, err := h.extractMetadata(data)
	if err != nil {
		h.failedCount++
		return err
	}

	log.Printf("Screenshot from device: %s at %s", metadata.DeviceName, metadata.Timestamp)
	if metadata.ActiveProcessName != "" {
		log.Printf("Active application: %s (%s)", metadata.ActiveProcessName, metadata.ActiveWindowTitle)
	}

	// TODO: Get actual image data (from S3 or base64 in message)
	// For now, we'll analyze based on metadata only
	imageData := h.getImageData(data)

	// Analyze screenshot using LLM
	analysis, err := h.analyzeScreenshot(metadata, imageData)
	if err != nil {
		h.failedCount++
		log.Printf("Failed to analyze screenshot: %v", err)
		return err
	}

	// Log analysis results
	h.logAnalysis(metadata, analysis)

	// Store in Qdrant vector database
	if h.qdrantClient != nil {
		if err := h.storeInVectorDB(analysis, metadata); err != nil {
			log.Printf("Warning: Failed to store in Qdrant: %v", err)
			// Don't fail the whole process if vector storage fails
		}
	} else {
		log.Println("Qdrant client not available, skipping vector storage")
	}

	h.processedCount++
	log.Printf("Screenshot processed successfully. Total processed: %d", h.processedCount)

	return nil
}

// extractMetadata extracts and validates screenshot metadata
func (h *ScreenshotHandler) extractMetadata(data map[string]interface{}) (*ScreenshotMetadata, error) {
	metadata := &ScreenshotMetadata{}

	// Required fields
	deviceName, ok := data["device_name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid device_name")
	}
	metadata.DeviceName = deviceName

	timestamp, ok := data["timestamp"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid timestamp")
	}
	metadata.Timestamp = timestamp

	// Optional fields
	if title, ok := data["active_window_title"].(string); ok {
		metadata.ActiveWindowTitle = title
	}

	if process, ok := data["active_process_name"].(string); ok {
		metadata.ActiveProcessName = process
	}

	if pid, ok := data["active_process_id"].(float64); ok {
		metadata.ActiveProcessID = int(pid)
	}

	if size, ok := data["image_size"].(float64); ok {
		metadata.ImageSize = int(size)
	}

	return metadata, nil
}

// getImageData retrieves image data from the message
func (h *ScreenshotHandler) getImageData(data map[string]interface{}) string {
	// Check for base64 encoded image
	if imageData, ok := data["image_data"].(string); ok {
		return imageData
	}

	// Check for S3 URL
	if imageURL, ok := data["image_url"].(string); ok {
		// TODO: Download from S3 and convert to base64
		log.Printf("Image URL: %s (download not implemented yet)", imageURL)
	}

	return ""
}

// analyzeScreenshot uses LLM to analyze the screenshot
func (h *ScreenshotHandler) analyzeScreenshot(metadata *ScreenshotMetadata, imageData string) (*ScreenshotAnalysis, error) {
	// Build analysis prompt
	prompt := h.buildAnalysisPrompt(metadata, imageData)

	// Create LLM request
	req := model.ChatCompletionRequest{
		Provider:    "openrouter",
		Model:       h.modelName,
		APIKey:      h.apiKey,
		Temperature: 0.3,
		MaxTokens:   1000,
		Messages: []model.ChatMessage{
			{
				Role:    "system",
				Content: getSystemPrompt(),
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	// Send request to LLM
	response, err := h.llmService.SendChatCompletion(req)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	// Parse response into structured analysis
	analysis := h.parseAnalysisResponse(response.Message.Content, metadata)

	return analysis, nil
}

// buildAnalysisPrompt creates the analysis prompt
func (h *ScreenshotHandler) buildAnalysisPrompt(metadata *ScreenshotMetadata, imageData string) string {
	prompt := fmt.Sprintf(`Analyze this screenshot activity based on the visual content and metadata.

Device: %s
Timestamp: %s
Active Application: %s
Window Title: %s

`, metadata.DeviceName, metadata.Timestamp, metadata.ActiveProcessName, metadata.ActiveWindowTitle)

	if imageData != "" {
		prompt += "Image data is available. Use visual analysis to answer the following based on the context.\n\n"
	}

	prompt += `Determine the context and strictly follow the relevant criteria:

1. **GAMING**:
   - **Activity**: What is the player doing? (e.g., combat, questing, inventory management)
   - **HUD/Stats**: List health, ammo, resources, level, and active effects.
   - **World**: Describe enemies, NPCs, dialogue options, and the environment.
   - **Identity**: Identify the game genre and character details.

2. **CODING & TECH**:
   - **Work**: What is the user writing or viewing? (e.g., implementing a class, debugging error logs)
   - **Code**: Identify the programming language, file extension, and code quality/complexity.
   - **Tools**: What IDE/Editor is used? Are terminal/debug/file-explorer panels visible?

3. **WEB BROWSING**:
   - **Page**: What is the specific content? (e.g., documentation, social feed, checkout page)
   - **Tabs**: List visible tabs and infer the browsing session goal.
   - **Action**: Is the user reading, typing, watching, or scrolling?

4. **VIDEO & ENTERTAINMENT**:
   - **Content**: What is being watched? Describe the scene or topic.
   - **Platform**: Layout (YouTube, Netflix, Twitch).
   - **Context**: How many videos/thumbnails are visible? Is the user engaging (comments/chat)?

5. **GENERAL**:
   - Describe the desktop state, open windows, and overall focus.

Output JSON:
{
  "summary": "Comprehensive 80-150 word description covering the specific criteria above.",
  "application": "Exact application name",
  "activity": "Specific action (e.g., 'Debugging main.go', 'Fighting boss in Elden Ring')",
  "time_visible": "Time on screen (or null)",
  "user_state": "Focused / Distracted / Multitasking / Idle",
  "productivity_level": "High / Medium / Low",
  "category": "Development / Browsing / Gaming / Entertainment / Communication / Other",
  "observations": "Key details (e.g., 'HP: 100/100', 'File: api.ts', 'Video: Rust Tutorial')"
}`

	return prompt
}

// parseAnalysisResponse parses LLM response into structured data
func (h *ScreenshotHandler) parseAnalysisResponse(content string, metadata *ScreenshotMetadata) *ScreenshotAnalysis {

	// Extract JSON from response (handle markdown code blocks)
	jsonStr := extractJSON(content)

	var llmResponse struct {
		Summary           string `json:"summary"`
		Application       string `json:"application"`
		Activity          string `json:"activity"`
		TimeVisible       string `json:"time_visible"`
		UserState         string `json:"user_state"`
		ProductivityLevel string `json:"productivity_level"`
		Category          string `json:"category"`
		Observations      string `json:"observations"`
	}

	// Try to parse JSON
	if err := json.Unmarshal([]byte(jsonStr), &llmResponse); err != nil {
		log.Printf("Failed to parse LLM JSON response: %v", err)
		// Fallback to basic analysis
		return &ScreenshotAnalysis{
			Application:       metadata.ActiveProcessName,
			Activity:          inferActivity(metadata.ActiveProcessName, metadata.ActiveWindowTitle),
			UserState:         "focused",
			ProductivityLevel: "medium",
			Category:          categorizeActivity(metadata.ActiveProcessName),
			TimeVisible:       metadata.Timestamp,
			Summary:           generateFallbackSummary(metadata),
			RawLLMResponse:    content,
			ConfidenceScore:   0.60,
		}
	}

	// Return parsed analysis
	return &ScreenshotAnalysis{
		Application:       llmResponse.Application,
		Activity:          llmResponse.Activity,
		UserState:         llmResponse.UserState,
		ProductivityLevel: llmResponse.ProductivityLevel,
		Category:          llmResponse.Category,
		TimeVisible:       llmResponse.TimeVisible,
		Summary:           llmResponse.Summary,
		Observations:      llmResponse.Observations,
		RawLLMResponse:    content,
		ConfidenceScore:   0.95,
	}
}

func generateFallbackSummary(metadata *ScreenshotMetadata) string {

	if metadata.ActiveProcessName == "" {
		return fmt.Sprintf("User activity captured at %s on device %s. No active application detected at this time.",
			metadata.Timestamp, metadata.DeviceName)
	}

	return fmt.Sprintf("User is working with %s on device %s at %s. The active window shows '%s'. This appears to be a %s activity session.",
		metadata.ActiveProcessName,
		metadata.DeviceName,
		metadata.Timestamp,
		metadata.ActiveWindowTitle,
		categorizeActivity(metadata.ActiveProcessName))
}

// logAnalysis logs the analysis results
func (h *ScreenshotHandler) logAnalysis(metadata *ScreenshotMetadata, analysis *ScreenshotAnalysis) {
	log.Printf("=== Screenshot Analysis ===")
	log.Printf("Device: %s", metadata.DeviceName)
	log.Printf("Application: %s", analysis.Application)
	log.Printf("Activity: %s", analysis.Activity)
	log.Printf("User State: %s", analysis.UserState)
	log.Printf("Category: %s", analysis.Category)
	log.Printf("Productivity: %s", analysis.ProductivityLevel)
	log.Printf("Summary: %s", analysis.Summary)
	log.Printf("Observations: %s", analysis.Observations)
	log.Printf("Confidence: %.2f", analysis.ConfidenceScore)
	log.Printf("==========================")
}

// GetStats returns handler statistics
func (h *ScreenshotHandler) GetStats() map[string]int {
	return map[string]int{
		"processed": h.processedCount,
		"failed":    h.failedCount,
	}
}

// Close closes the handler and cleans up resources
func (h *ScreenshotHandler) Close() error {
	if h.qdrantClient != nil {
		return h.qdrantClient.Close()
	}
	return nil
}

// storeInVectorDB stores the analysis in Qdrant vector database
func (h *ScreenshotHandler) storeInVectorDB(
	analysis *ScreenshotAnalysis,
	metadata *ScreenshotMetadata,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Ensure collection exists before upserting
	vectorSize := vectorstore.GetVectorSize()
	if err := h.qdrantClient.EnsureCollection(ctx, vectorSize); err != nil {
		return fmt.Errorf("failed to ensure collection: %w", err)
	}

	// Create text for embedding
	textToEmbed := fmt.Sprintf(
		"%s. Application: %s. Activity: %s. Category: %s. User State: %s. Observations: %s.",
		analysis.Summary,
		analysis.Application,
		analysis.Activity,
		analysis.Category,
		analysis.UserState,
		analysis.Observations,
	)

	// Generate embedding
	embedding, err := h.generateEmbedding(textToEmbed)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Create memory
	memory := vectorstore.ScreenshotMemory{
		ID:        uuid.New().String(),
		Summary:   analysis.Summary,
		Embedding: embedding,
		Metadata: map[string]interface{}{
			"application":        analysis.Application,
			"activity":           analysis.Activity,
			"category":           analysis.Category,
			"user_state":         analysis.UserState,
			"productivity_level": analysis.ProductivityLevel,
			"device":             metadata.DeviceName,
			"process_name":       metadata.ActiveProcessName,
			"active_window":      metadata.ActiveWindowTitle,
			"confidence_score":   analysis.ConfidenceScore,
			"timestamp":          metadata.Timestamp,
			"observations":       analysis.Observations,
		},
		Timestamp: time.Now(),
	}

	// Store in Qdrant
	if err := h.qdrantClient.UpsertMemory(ctx, memory); err != nil {
		return fmt.Errorf("failed to upsert memory: %w", err)
	}

	log.Printf("✓ Successfully stored in Qdrant: %s", memory.ID)
	return nil
}

// generateEmbedding generates a vector embedding for the text
// TODO: Replace with actual embedding service (OpenAI, Ollama, etc.)
func (h *ScreenshotHandler) generateEmbedding(text string) ([]float32, error) {
	// For now, generate a mock embedding
	// In production, replace this with actual embedding generation
	vectorSize := int(vectorstore.GetVectorSize())
	embedding := make([]float32, vectorSize)

	// Simple hash-based mock embedding (deterministic)
	hash := 0
	for _, char := range text {
		hash = (hash*31 + int(char)) % 1000
	}

	for i := range embedding {
		embedding[i] = float32(hash%100) / 100.0
	}

	log.Printf("Generated mock embedding (size: %d) - Replace with real embedding service!", vectorSize)
	return embedding, nil
}

// Helper types

type ScreenshotMetadata struct {
	DeviceName        string
	Timestamp         string
	ActiveWindowTitle string
	ActiveProcessName string
	ActiveProcessID   int
	ImageSize         int
}

type ScreenshotAnalysis struct {
	Application       string
	Activity          string
	UserState         string
	ProductivityLevel string
	Category          string
	TimeVisible       string
	Summary           string // 50-80 word description of what's happening
	Observations      string
	RawLLMResponse    string
	ConfidenceScore   float64
}

// Helper functions (DRY principle)

func getSystemPrompt() string {
	return `You are an expert at analyzing user activity from screenshots and application metadata.
Your goal is to understand what the user is doing, their focus level, and categorize their activity.
Provide accurate, concise analysis in JSON format.`
}

func inferActivity(processName, windowTitle string) string {
	// Simple activity inference based on process and window
	if processName == "" {
		return "unknown"
	}

	// Add more sophisticated logic here
	switch {
	case contains(processName, "chrome", "firefox", "edge"):
		return fmt.Sprintf("Browsing: %s", windowTitle)
	case contains(processName, "code", "visual studio"):
		return "Coding"
	case contains(processName, "word", "excel", "powerpoint"):
		return "Document editing"
	case contains(processName, "slack", "teams", "discord"):
		return "Communication"
	default:
		return fmt.Sprintf("Using %s", processName)
	}
}

func categorizeActivity(processName string) string {
	switch {
	case contains(processName, "code", "visual studio", "intellij", "pycharm"):
		return "development"
	case contains(processName, "chrome", "firefox", "edge", "safari"):
		return "browsing"
	case contains(processName, "slack", "teams", "discord", "zoom"):
		return "communication"
	case contains(processName, "word", "excel", "powerpoint", "notion"):
		return "documentation"
	case contains(processName, "spotify", "youtube", "netflix"):
		return "entertainment"
	default:
		return "other"
	}
}

func contains(str string, substrs ...string) bool {
	str = toLower(str)
	for _, substr := range substrs {
		if containsSubstr(str, toLower(substr)) {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	return strings.ToLower(s)
}

func containsSubstr(s, substr string) bool {
	return strings.Contains(s, substr)
}
