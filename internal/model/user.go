package model

import (
	"time"
)

type User struct {
	ID        string    `bson:"_id,omitempty" json:"_id,omitempty"`
	Name      string    `bson:"name" validate:"required" json:"name"`
	Username  string    `bson:"username" validate:"required" json:"username"`
	Email     string    `bson:"email" validate:"required" json:"email"`
	Avatar    string    `bson:"avatar" json:"avatar"`
	Password  string    `bson:"password" validate:"required" json:"password"`
	Phone     string    `bson:"phone" json:"phone"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}
