package model

import (
	"time"
)

type User struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	Username  string    `bson:"username"`
	Email     string    `bson:"email"`
	Avatar    string    `bson:"avatar"`
	Password  string    `bson:"password"`
	Phone     string    `bson:"phone"`
	CreatedAt time.Time `bson:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt"`
}
