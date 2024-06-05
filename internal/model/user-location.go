package model

import "time"

type UserLocation struct {
	ID         string    `bson:"_id,omitempty" json:"_id,omitempty"`
	UserID     string    `bson:"userId" json:"userId"`
	TripID     string    `bson:"tripId" json:"tripId"`
	CurrentLat string    `bson:"currentLat" json:"currentLat"`
	CurrentLng string    `bson:"currentLng" json:"currentLng"`
	StartLat   string    `bson:"startLat" json:"startLat"`
	StartLng   string    `bson:"startLng" json:"startLng"`
	LastLat    string    `bson:"lastLat" json:"lastLat"`
	LastLng    string    `bson:"lastLng" json:"lastLng"`
	EndedAt    time.Time `bson:"endedAt" json:"endedAt"`
	IsDeleted  bool      `bson:"isDeleted" json:"isDeleted"`
	IsStarted  bool      `bson:"isStarted" json:"isStarted"`
	CreatedAt  time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time `bson:"updatedAt" json:"updatedAt"`
}
