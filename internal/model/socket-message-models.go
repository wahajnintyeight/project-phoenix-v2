package model

type IdentifyUser struct {
	UserId string `json:"userId"`
	TripId string `json:"tripId"`
}

type LocationData struct {
	UserId     string  `json:"userId"`
	TripId     string  `json:"tripId"`
	CurrentLat float64 `json:"currentLat"`
	CurrentLng float64 `json:"currentLng"`
}
