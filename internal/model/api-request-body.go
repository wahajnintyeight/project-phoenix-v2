package model

type RegisterModel struct {
	User
	ConfirmPassword string `json:"confirmPassword"`
}

type LoginModel struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FcmKey   string `json:"fcmKey"`
}
