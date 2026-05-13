package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Status constants
const (
	StatusPending        = "Pending"
	StatusValid          = "Valid"
	StatusValidNoCredits = "ValidNoCredits"
	StatusInvalid        = "Invalid"
	StatusError          = "Error"
)

// Provider constants
const (
	ProviderOpenAI      = "OpenAI"
	ProviderAnthropic   = "Anthropic"
	ProviderGoogle      = "Google"
	ProviderOpenRouter  = "OpenRouter"
	ProviderMoonshot    = "Moonshot"
	ProviderHuggingFace = "HuggingFace"
	ProviderDeepSeek    = "DeepSeek"
)

type APIKey struct {
	ID          primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	KeyValue    string                 `bson:"key_value" json:"key_value"`
	Provider    string                 `bson:"provider" json:"provider"`
	Status      string                 `bson:"status" json:"status"`
	CreatedAt   time.Time              `bson:"created_at" json:"created_at"`
	ValidatedAt *time.Time             `bson:"validated_at,omitempty" json:"validated_at,omitempty"`
	LastSeenAt  time.Time              `bson:"last_seen_at" json:"last_seen_at"`
	NotifiedAt  *time.Time             `bson:"notified_at,omitempty" json:"notified_at,omitempty"` // Track when Discord notification was sent
	ErrorCount  int                    `bson:"error_count" json:"error_count"`
	RepoRefs    []primitive.ObjectID   `bson:"repo_refs" json:"repo_refs"`
	Credits     map[string]interface{} `bson:"credits,omitempty" json:"credits,omitempty"` // Store provider-specific credits info
}

// APIKeyWithReferences includes the API key and its populated repo references
type APIKeyWithReferences struct {
	APIKey
	References []*RepoReference `json:"references"`
}
