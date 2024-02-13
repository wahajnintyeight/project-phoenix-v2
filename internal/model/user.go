package model

import (
	"time"
)

type User struct {
	ID        string    `bson:"_id,omitempty" json:"id,omitempty"`
	Name      string    `bson:"name" validate:"required"`
	Username  string    `bson:"username"`
	Email     string    `bson:"email" validate:"required"`
	Avatar    string    `bson:"avatar"`
	Password  string    `bson:"password" validate:"required"`
	Phone     string    `bson:"phone"`
	CreatedAt time.Time `bson:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt"`
}
