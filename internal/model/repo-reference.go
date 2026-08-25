package model

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RepoReference struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	APIKeyID primitive.ObjectID `bson:"api_key_id" json:"api_key_id"`
	FileURL  string             `bson:"file_url" json:"file_url"`
}
