package model

type RegisterModel struct {
	User
	ConfirmPassword string `json:"confirmPassword"`
}
