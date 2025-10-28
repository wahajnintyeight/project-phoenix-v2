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
	FcmToken string `json:"fcmKey"`
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

type CaptureScreenDeviceQueryModel struct {
	DeviceName string `json:"deviceName"`
}

type ClipboardRequestModel struct {
	Code string `json:"code" bson:"code"`
	DeviceInfo   string  `json:"deviceInfo" bson:"deviceInfo"`
}

type ClipboardUpdateNameRequestModel struct {
	Code string `json:"code" bson:"code"`
	RoomName string `json:"roomName" bson:"roomName"`
}

type GoogleSearchVideoRequestModel struct {
	Query string `json:"query" bson:"query"`
	MaxResults int `json:"maxResults" bson:"maxResults"`
	NextPage string `json:"nextPage" bson:"nextPage"`
	PrevPage string `json:"prevPage" bson:"prevPage"`
}

type GoogleDownloadVideoRequestModel struct {
	VideoId string `json:"videoId" bson:"videoId"`
	DownloadId string `json:"downloadId" bson:"downloadId"`
	YoutubeURL string `json:"youtubeURL" bson:"youtubeURL"`
	Format string  `json:"format" bson:"format"`
	BitRate string `json:"bitRate" bson:"bitRate"`
}
