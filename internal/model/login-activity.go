package model

import "time"

type LoginActivity struct {
	UserID      string    `bson:"userId" json:"userId"`
	UserAgent   string    `bson:"userAgent" json:"userAgent"`
	IPAddress   string    `bson:"ipAddress" json:"ipAddress"`
	Token       string    `bson:"token" json:"token"`
	GoogleToken string    `bson:"googleToken" json:"googleToken"`
	FCMKey      string    `bson:"fcmKey" json:"fcmKey"`
	SessionID   string    `bson:"sessionId" json:"sessionId"`
	CreatedAt   time.Time `bson:"createdAt" json:"createdAt"`
	UpdateAt    time.Time `bson:"updatedAt" json:"updatedAt"`
	IsRider     bool      `bson:"isRider" json:"isRider"`
	IsSpectator bool      `bson:"isSpectator" json:"isSpectator"`
	DeviceName  string    `bson:"deviceName" json:"deviceName"`
	IP          string    `bson:"ip" json:"ip"`
}
