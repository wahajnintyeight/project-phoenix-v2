package model

import "time"

type UserTrip struct {
	ID                     string     `bson:"_id,omitempty" json:"_id,omitempty"`
	UserID                 string     `bson:"userId" json:"userId"`
	TripID                 string     `bson:"tripId" json:"tripId"`
	Name                   string     `bson:"name" json:"name"`
	IsDeleted              bool       `bson:"isDeleted" json:"isDeleted"`
	UpdatedAt              time.Time  `bson:"updatedAt" json:"updatedAt"`
	CreatedAt              time.Time  `bson:"createdAt" json:"createdAt"`
	IsNotificationsEnabled bool       `bson:"isNotificationsEnabled" json:"isNotificationsEnabled"`
	IsStarted              bool       `bson:"isStarted" json:"isStarted"`
	LastStarted            *time.Time `bson:"lastStarted,omitempty" json:"lastStarted,omitempty"`
	CurrentLat             string     `bson:"currentLat" json:"currentLat"`
	CurrentLng             string     `bson:"currentLng" json:"currentLng"`
}
