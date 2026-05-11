package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Visit struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	IP          string             `bson:"ip" json:"ip"`
	Country     string             `bson:"country" json:"country"`
	CountryCode string             `bson:"country_code" json:"country_code"`
	ProjectType string             `bson:"project_type" json:"project_type"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}
