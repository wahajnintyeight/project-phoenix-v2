package model

import "time"

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

type ClipBoardRoomJoined struct {
	Code           string 			       `json:"code" bson:"code"`
	DeviceInfo 	   ClipBoardRoomDeviceInfo `json:"deviceInfo" bson:"deviceInfo"`
	IsAnonymous    bool 				   `json:"isAnonymous" bson:"isAnonymous"`
	UserId         string				   `json:"userId" bson:"userId"`  
}

type ClipBoardSendRoomMessage struct {
	RoomID             string    				 `json:"roomId"         bson:"roomId"`
	Sender    	   	   string    				 `json:"sender"         bson:"sender"`
	Message            string    				 `json:"message"        bson:"message"`
	Code               string    				 `json:"code"           bson:"code"`
	TimeStamp          time.Time 			     `json:"timeStamp"      bson:"timeStamp"`
	IsAttachment       bool      				 `json:"isAttachment"   bson:"isAttachment"`
	AttachmentType     string    			     `json:"attachmentType" bson:"attachmentType"`
	AttachmentURL      string    				 `json:"attachmentURL"  bson:"attachmentURL"`
	DeviceInfo         ClipBoardRoomDeviceInfo   `json:"deviceInfo"     bson:"deviceInfo"`
	IsAnonymous        bool                      `json:"isAnonymous"    bson:"isAnonymous"`
}

type ClipBoardRoomDeviceInfo struct {
	SlugifiedDeviceName string `json:"slugifiedDeviceName"`
	DeviceName string `json:"deviceName"`
}