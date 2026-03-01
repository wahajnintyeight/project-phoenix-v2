package vectorstore

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type QdrantClient struct {
	conn           *grpc.ClientConn
	collections    pb.CollectionsClient
	points         pb.PointsClient
	collectionName string
}

type ScreenshotMemory struct {
	ID        string
	Summary   string
	Embedding []float32
	Metadata  map[string]interface{}
	Timestamp time.Time
}

// NewQdrantClient creates a new Qdrant client
func NewQdrantClient() (*QdrantClient, error) {
	godotenv.Load()

	host := getEnvOrDefault("QDRANT_HOST", "localhost")
	port := getEnvOrDefault("QDRANT_PORT", "6334")
	collectionName := getEnvOrDefault("QDRANT_COLLECTION_NAME", "screenshot_memories")
	apiKey := os.Getenv("QDRANT_API_KEY")

	addr := fmt.Sprintf("%s:%s", host, port)
	log.Printf("Connecting to Qdrant at %s", addr)

	// Configure connection options
	opts := []grpc.DialOption{}

	if apiKey != "" {
		// Use TLS with API key for Qdrant Cloud
		log.Printf("Using API key authentication for Qdrant Cloud")
		opts = append(opts,
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
			grpc.WithPerRPCCredentials(&apiKeyCredentials{apiKey: apiKey}),
		)
	} else {
		// Use insecure connection for local Qdrant
		log.Printf("Using insecure connection for local Qdrant")
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant: %w", err)
	}

	client := &QdrantClient{
		conn:           conn,
		collections:    pb.NewCollectionsClient(conn),
		points:         pb.NewPointsClient(conn),
		collectionName: collectionName,
	}

	log.Printf("Successfully connected to Qdrant collection: %s", collectionName)
	return client, nil
}

// Close closes the Qdrant connection
func (c *QdrantClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// CreateCollection creates a new collection with vector configuration
func (c *QdrantClient) CreateCollection(ctx context.Context, vectorSize uint64) error {
	log.Printf("Creating collection '%s' with vector size %d", c.collectionName, vectorSize)

	_, err := c.collections.Create(ctx, &pb.CreateCollection{
		CollectionName: c.collectionName,
		VectorsConfig: &pb.VectorsConfig{
			Config: &pb.VectorsConfig_Params{
				Params: &pb.VectorParams{
					Size:     vectorSize,
					Distance: pb.Distance_Cosine,
				},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	log.Printf("Collection '%s' created successfully", c.collectionName)
	return nil
}

// CollectionExists checks if the collection exists
func (c *QdrantClient) CollectionExists(ctx context.Context) (bool, error) {
	resp, err := c.collections.CollectionExists(ctx, &pb.CollectionExistsRequest{
		CollectionName: c.collectionName,
	})
	if err != nil {
		return false, fmt.Errorf("failed to check collection existence: %w", err)
	}
	return resp.Result.Exists, nil
}

// UpsertMemory inserts or updates a screenshot memory
func (c *QdrantClient) UpsertMemory(ctx context.Context, memory ScreenshotMemory) error {
	log.Printf("Upserting memory with ID: %s", memory.ID)

	// Build payload from metadata
	payload := c.buildPayload(memory)

	// Create point
	point := &pb.PointStruct{
		Id: &pb.PointId{
			PointIdOptions: &pb.PointId_Uuid{
				Uuid: memory.ID,
			},
		},
		Vectors: &pb.Vectors{
			VectorsOptions: &pb.Vectors_Vector{
				Vector: &pb.Vector{
					Data: memory.Embedding,
				},
			},
		},
		Payload: payload,
	}

	// Upsert the point
	_, err := c.points.Upsert(ctx, &pb.UpsertPoints{
		CollectionName: c.collectionName,
		Points:         []*pb.PointStruct{point},
	})

	if err != nil {
		return fmt.Errorf("failed to upsert memory: %w", err)
	}

	log.Printf("Memory upserted successfully: %s", memory.ID)
	return nil
}

// UpsertMemories batch inserts or updates multiple screenshot memories
func (c *QdrantClient) UpsertMemories(ctx context.Context, memories []ScreenshotMemory) error {
	if len(memories) == 0 {
		return nil
	}

	log.Printf("Batch upserting %d memories", len(memories))

	points := make([]*pb.PointStruct, 0, len(memories))

	for _, memory := range memories {
		payload := c.buildPayload(memory)

		point := &pb.PointStruct{
			Id: &pb.PointId{
				PointIdOptions: &pb.PointId_Uuid{
					Uuid: memory.ID,
				},
			},
			Vectors: &pb.Vectors{
				VectorsOptions: &pb.Vectors_Vector{
					Vector: &pb.Vector{
						Data: memory.Embedding,
					},
				},
			},
			Payload: payload,
		}

		points = append(points, point)
	}

	_, err := c.points.Upsert(ctx, &pb.UpsertPoints{
		CollectionName: c.collectionName,
		Points:         points,
	})

	if err != nil {
		return fmt.Errorf("failed to batch upsert memories: %w", err)
	}

	log.Printf("Batch upserted %d memories successfully", len(memories))
	return nil
}

// buildPayload converts memory metadata to Qdrant payload format
func (c *QdrantClient) buildPayload(memory ScreenshotMemory) map[string]*pb.Value {
	payload := map[string]*pb.Value{
		"summary": {
			Kind: &pb.Value_StringValue{
				StringValue: memory.Summary,
			},
		},
		"timestamp": {
			Kind: &pb.Value_StringValue{
				StringValue: memory.Timestamp.Format(time.RFC3339),
			},
		},
	}

	// Add metadata fields
	for key, value := range memory.Metadata {
		payload[key] = convertToQdrantValue(value)
	}

	return payload
}

// convertToQdrantValue converts Go types to Qdrant Value types
func convertToQdrantValue(value interface{}) *pb.Value {
	switch v := value.(type) {
	case string:
		return &pb.Value{
			Kind: &pb.Value_StringValue{
				StringValue: v,
			},
		}
	case int:
		return &pb.Value{
			Kind: &pb.Value_IntegerValue{
				IntegerValue: int64(v),
			},
		}
	case int64:
		return &pb.Value{
			Kind: &pb.Value_IntegerValue{
				IntegerValue: v,
			},
		}
	case float64:
		return &pb.Value{
			Kind: &pb.Value_DoubleValue{
				DoubleValue: v,
			},
		}
	case float32:
		return &pb.Value{
			Kind: &pb.Value_DoubleValue{
				DoubleValue: float64(v),
			},
		}
	case bool:
		return &pb.Value{
			Kind: &pb.Value_BoolValue{
				BoolValue: v,
			},
		}
	default:
		// Fallback to string representation
		return &pb.Value{
			Kind: &pb.Value_StringValue{
				StringValue: fmt.Sprintf("%v", v),
			},
		}
	}
}

// Helper function to get environment variable with default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetCollectionInfo returns information about the collection
func (c *QdrantClient) GetCollectionInfo(ctx context.Context) (*pb.CollectionInfo, error) {
	resp, err := c.collections.Get(ctx, &pb.GetCollectionInfoRequest{
		CollectionName: c.collectionName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get collection info: %w", err)
	}
	return resp.Result, nil
}

// DeleteMemory deletes a memory by ID
func (c *QdrantClient) DeleteMemory(ctx context.Context, id string) error {
	log.Printf("Deleting memory with ID: %s", id)

	_, err := c.points.Delete(ctx, &pb.DeletePoints{
		CollectionName: c.collectionName,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Points{
				Points: &pb.PointsIdsList{
					Ids: []*pb.PointId{
						{
							PointIdOptions: &pb.PointId_Uuid{
								Uuid: id,
							},
						},
					},
				},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to delete memory: %w", err)
	}

	log.Printf("Memory deleted successfully: %s", id)
	return nil
}

// CountPoints returns the number of points in the collection
func (c *QdrantClient) CountPoints(ctx context.Context) (uint64, error) {
	resp, err := c.points.Count(ctx, &pb.CountPoints{
		CollectionName: c.collectionName,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to count points: %w", err)
	}
	return resp.Result.Count, nil
}

// EnsureCollection creates the collection if it doesn't exist
func (c *QdrantClient) EnsureCollection(ctx context.Context, vectorSize uint64) error {
	exists, err := c.CollectionExists(ctx)
	if err != nil {
		log.Printf("❌ Error checking if collection exists: %v", err)
		return err
	}

	if !exists {
		log.Printf("📦 Collection '%s' does not exist, creating with vector size %d...", c.collectionName, vectorSize)
		if err := c.CreateCollection(ctx, vectorSize); err != nil {
			log.Printf("❌ Failed to create collection: %v", err)
			return err
		}
		log.Printf("✓ Collection '%s' created successfully", c.collectionName)
	} else {
		log.Printf("✓ Collection '%s' already exists", c.collectionName)
	}

	return nil
}

// GetVectorSize returns the configured vector size from environment or default
func GetVectorSize() uint64 {
	sizeStr := getEnvOrDefault("QDRANT_VECTOR_SIZE", "1536")
	size, err := strconv.ParseUint(sizeStr, 10, 64)
	if err != nil {
		log.Printf("Invalid QDRANT_VECTOR_SIZE, using default 1536: %v", err)
		return 1536
	}
	return size
}

// apiKeyCredentials implements credentials.PerRPCCredentials for API key authentication
type apiKeyCredentials struct {
	apiKey string
}

func (c *apiKeyCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"api-key": c.apiKey,
	}, nil
}

func (c *apiKeyCredentials) RequireTransportSecurity() bool {
	return true
}
