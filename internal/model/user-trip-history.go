package model

import "time"

type UserTripHistory struct {
	ID             string    `bson:"_id,omitempty" json:"_id,omitempty"`
	UserID         string    `bson:"userId" json:"userId"`
	TripID         string    `bson:"tripId" json:"tripId"`
	UserLocationID string    `bson:"userLocationId" json:"userLocationId"`
	Lat            string    `bson:"lat" json:"lat"`
	Lng            string    `bson:"lng" json:"lng"`
	StartedAt      time.Time `bson:"startedAt" json:"startedAt"`
	IsDeleted      bool      `bson:"isDeleted" json:"isDeleted"`
	EndedAt        time.Time `bson:"endedAt" json:"endedAt"`
	CreatedAt      time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time `bson:"updatedAt" json:"updatedAt"`
}
