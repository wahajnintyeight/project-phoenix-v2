package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SearchQuery struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	QueryPattern   string             `bson:"query_pattern" json:"query_pattern"`
	Provider       string             `bson:"provider" json:"provider"`
	Enabled        bool               `bson:"enabled" json:"enabled"`
	LastSearchedAt *time.Time         `bson:"last_searched_at,omitempty" json:"last_searched_at,omitempty"`
	ResultCount    int                `bson:"result_count" json:"result_count"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
}
