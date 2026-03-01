package vectorstore

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// ExampleUsage demonstrates how to use the Qdrant client
func ExampleUsage() {
	ctx := context.Background()

	// 1. Create a new Qdrant client
	client, err := NewQdrantClient()
	if err != nil {
		log.Fatalf("Failed to create Qdrant client: %v", err)
	}
	defer client.Close()

	// 2. Ensure collection exists (creates if not exists)
	vectorSize := GetVectorSize() // Default 1536 for OpenAI embeddings
	if err := client.EnsureCollection(ctx, vectorSize); err != nil {
		log.Fatalf("Failed to ensure collection: %v", err)
	}

	// 3. Create a sample memory (you would get the embedding from your embedding service)
	memory := ScreenshotMemory{
		ID:        uuid.New().String(),
		Summary:   "User is coding in Visual Studio Code, working on a Go project",
		Embedding: generateMockEmbedding(int(vectorSize)), // Replace with actual embedding
		Metadata: map[string]interface{}{
			"application":        "Visual Studio Code",
			"activity":           "Coding",
			"category":           "development",
			"user_state":         "focused",
			"productivity":       "high",
			"device":             "DESKTOP-ABC123",
			"process_name":       "Code.exe",
			"active_window":      "main.go - project-phoenix-v2",
			"productivity_level": 0.95,
		},
		Timestamp: time.Now(),
	}

	// 4. Upsert single memory
	if err := client.UpsertMemory(ctx, memory); err != nil {
		log.Fatalf("Failed to upsert memory: %v", err)
	}
	log.Println("Single memory upserted successfully")

	// 5. Batch upsert multiple memories
	memories := []ScreenshotMemory{
		{
			ID:        uuid.New().String(),
			Summary:   "User browsing documentation on MDN Web Docs",
			Embedding: generateMockEmbedding(int(vectorSize)),
			Metadata: map[string]interface{}{
				"application":   "Google Chrome",
				"activity":      "Reading documentation",
				"category":      "learning",
				"user_state":    "focused",
				"productivity":  "medium",
				"device":        "DESKTOP-ABC123",
				"process_name":  "chrome.exe",
				"active_window": "JavaScript | MDN",
			},
			Timestamp: time.Now().Add(-5 * time.Minute),
		},
		{
			ID:        uuid.New().String(),
			Summary:   "User watching YouTube video on Go programming",
			Embedding: generateMockEmbedding(int(vectorSize)),
			Metadata: map[string]interface{}{
				"application":   "Google Chrome",
				"activity":      "Watching video",
				"category":      "entertainment",
				"user_state":    "distracted",
				"productivity":  "low",
				"device":        "DESKTOP-ABC123",
				"process_name":  "chrome.exe",
				"active_window": "Learn Go in 10 Minutes - YouTube",
			},
			Timestamp: time.Now().Add(-10 * time.Minute),
		},
	}

	if err := client.UpsertMemories(ctx, memories); err != nil {
		log.Fatalf("Failed to batch upsert memories: %v", err)
	}
	log.Printf("Batch upserted %d memories successfully", len(memories))

	// 6. Get collection info
	info, err := client.GetCollectionInfo(ctx)
	if err != nil {
		log.Fatalf("Failed to get collection info: %v", err)
	}
	log.Printf("Collection info: %+v", info)

	// 7. Count points
	count, err := client.CountPoints(ctx)
	if err != nil {
		log.Fatalf("Failed to count points: %v", err)
	}
	log.Printf("Total points in collection: %d", count)

	// 8. Delete a memory (optional)
	// if err := client.DeleteMemory(ctx, memory.ID); err != nil {
	// 	log.Fatalf("Failed to delete memory: %v", err)
	// }
	// log.Println("Memory deleted successfully")
}

// generateMockEmbedding creates a mock embedding vector
// In production, replace this with actual embeddings from OpenAI, Ollama, etc.
func generateMockEmbedding(size int) []float32 {
	embedding := make([]float32, size)
	for i := range embedding {
		embedding[i] = 0.1 // Mock value
	}
	return embedding
}

// IntegrationExample shows how to integrate with screenshot handler
func IntegrationExample(analysis *ScreenshotAnalysis, metadata *ScreenshotMetadata, embedding []float32) error {
	ctx := context.Background()

	// Create client
	client, err := NewQdrantClient()
	if err != nil {
		return fmt.Errorf("failed to create Qdrant client: %w", err)
	}
	defer client.Close()

	// Ensure collection exists
	if err := client.EnsureCollection(ctx, GetVectorSize()); err != nil {
		return fmt.Errorf("failed to ensure collection: %w", err)
	}

	// Create memory from analysis
	memory := ScreenshotMemory{
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
		},
		Timestamp: time.Now(),
	}

	// Store in Qdrant
	if err := client.UpsertMemory(ctx, memory); err != nil {
		return fmt.Errorf("failed to store memory: %w", err)
	}

	log.Printf("Successfully stored screenshot analysis in Qdrant: %s", memory.ID)
	return nil
}

// ScreenshotAnalysis and ScreenshotMetadata types for integration example
type ScreenshotAnalysis struct {
	Application       string
	Activity          string
	UserState         string
	ProductivityLevel string
	Category          string
	TimeVisible       string
	Summary           string
	RawLLMResponse    string
	ConfidenceScore   float64
}

type ScreenshotMetadata struct {
	DeviceName        string
	Timestamp         string
	ActiveWindowTitle string
	ActiveProcessName string
	ActiveProcessID   int
	ImageSize         int
}
