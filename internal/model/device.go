package model

import "time"

type Device struct {
	ID                     string     `bson:"_id,omitempty" json:"_id,omitempty"`
	DeviceName string    `json:"deviceName"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	IsOnline   bool      `json:"isOnline"`
	LastOnline time.Time `json:"lastOnline"`
	LastImage  string    `json:"lastImage"`
	MemoryUsage string `json:"memoryUsage"`
	DiskUsage string `json:"diskUsage"`
	OSName string `json:"osName"`
}
