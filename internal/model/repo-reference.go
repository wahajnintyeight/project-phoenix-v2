package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RepoReference struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	APIKeyID  primitive.ObjectID `bson:"api_key_id" json:"api_key_id"`
	RepoURL   string             `bson:"repo_url" json:"repo_url"`
	RepoOwner string             `bson:"repo_owner" json:"repo_owner"`
	RepoName  string             `bson:"repo_name" json:"repo_name"`
	FileURL   string             `bson:"file_url" json:"file_url"`
	FilePath  string             `bson:"file_path" json:"file_path"`
	FoundAt   time.Time          `bson:"found_at" json:"found_at"`
}
