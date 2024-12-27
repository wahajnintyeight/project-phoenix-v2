package model

import "time"

type Device struct {
	ID                     string     `bson:"_id,omitempty" json:"_id,omitempty"`
	DeviceName string    `json:"deviceName" bson:"deviceName"`
	CreatedAt  time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt" bson:"updatedAt"`
	IsOnline   bool      `json:"isOnline" bson:"isOnline"`
	LastOnline time.Time `json:"lastOnline" bson:"lastOnline"`
	LastImage  string    `json:"lastImage" bson:"lastImage"`
	MemoryUsage string `json:"memoryUsage" bson:"memoryUsage"`
	DiskUsage string `json:"diskUsage" bson:"diskUsage"`
	OSName string `json:"osName" bson:"osName"`
}
