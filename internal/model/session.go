package model

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"time"
)

type Session struct {
	SessionID string    `bson:"sessionId"`
	CreatedAt time.Time `bson:"createdAt"`
}


