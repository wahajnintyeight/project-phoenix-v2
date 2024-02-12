package model

import (
	"time"
)

type Session struct {
	SessionID string    `bson:"sessionId"`
	CreatedAt time.Time `bson:"createdAt"`
}
