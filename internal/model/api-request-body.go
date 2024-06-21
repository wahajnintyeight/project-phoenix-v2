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

type GoogleLoginModel struct {
	Token string `json:"token"`
}

type GoogleUserModel struct {
	AuthTime int64  `json:"auth_time"`
	Iss      string `json:"iss"`
	Aud      string `json:"aud"`
	Exp      int64  `json:"exp"`
	Iat      int64  `json:"iat"`
	Sub      string `json:"sub"`
	Uid      string `json:"uid"`
	Firebase struct {
		SignInProvider string `json:"sign_in_provider"`
		Tenant         string `json:"tenant"`
		Identities     struct {
			Email     []string `json:"email"`
			GoogleCom []string `json:"google.com"`
		}
	}
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type StartTrackingModel struct {
	TripID     string  `json:"tripId"`
	CurrentLat float64 `json:"currentLat"`
	CurrentLng float64 `json:"currentLng"`
}

type StopTrackingModel struct {
	TripID     string  `json:"tripId"`
	CurrentLat float64 `json:"currentLat"`
	CurrentLng float64 `json:"currentLng"`
}

type CreateTripModel struct {
	TripName           string `json:"name"`
	EnableNotification bool   `json:"enableNotification"`
}

type DeleteTripModel struct {
	TripId string `json:"tripId"`
}

type GetCurrentLocationModel struct {
	TripID string `json:"tripId"`
}