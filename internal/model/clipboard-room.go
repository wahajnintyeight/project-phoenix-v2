package model

import "time"

type ClipboardRoom struct {
	ID            string                 `bson:"_id,omitempty" json:"_id,omitempty"`
	RoomName      string                 `json:"roomName" bson:"roomName"`
	CreatedAt     time.Time              `json:"createdAt" bson:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt" bson:"updatedAt"`
	TotalMessages int32                  `json:"totalMessages" bson:"totalMessages" default:"0"`
	LastMessage   time.Time              `json:"lastMessage" bson:"lastMessage"`
	Code          string          		 `json:"code" bson:"code"`
	Messages      []ClipboardRoomMessage `json:"messages" bson:"messages" default:"[]"`
	Members       []ClipboardRoomMember  `json:"members" bson:"members"`
}

type ClipboardRoomMessage struct {
	ID             string    `bson:"_id,omitempty" json:"_id,omitempty"`
	Message        string    `json:"message" bson:"message"`
	CreatedAt      time.Time `json:"createdAt" bson:"createdAt"`
	Sender         string    `json:"sender" bson:"sender"`
	IsAttachment   bool      `json:"isAttachment" bson:"isAttachment"`
	AttachmentType string    `json:"attachmentType" bson:"attachmentType"`
	AttachmentURL  string    `json:"attachmentURL" bson:"attachmentURL"`
}

type ClipboardRoomMember struct {
	ID             string    `bson:"_id,omitempty" json:"_id,omitempty"`
	IP             string    `bson:"ip" json:"ip"`
	UserAgent      string    `bson:"userAgent" json:"userAgent"`
	JoinedAt       time.Time `bson:"joinedAt" json:"joinedAt"`
}