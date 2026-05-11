package model

import (
	"time"
)

type Session struct {
	SessionID   string    `bson:"sessionID" json:"session_id"`
	CreatedAt   time.Time `bson:"createdAt" json:"created_at"`
	IP          string    `bson:"ip" json:"ip"`
	ProjectType string    `bson:"projectType" json:"project_type"`
}
