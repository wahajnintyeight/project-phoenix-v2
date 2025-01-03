package model

import "time"

type ClipboardRoom struct {
	ID            string                 `bson:"_id,omitempty" json:"_id,omitempty"`
	RoomName      string                 `json:"roomName" bson:"roomName"`
	CreatedAt     time.Time              `json:"createdAt" bson:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt" bson:"updatedAt"`
	TotalMessages int32                  `json:"totalMessages" bson:"totalMessages"`
	LastMessage   time.Time              `json:"lastMessage" bson:"lastMessage"`
	Messages      []ClipboardRoomMessage `json:"messages" bson:"messages"`
}

type ClipboardRoomMessage struct {
	ID             string    `bson:"_id,omitempty" json:"_id,omitempty"`
	Text           string    `json:"text" bson:"text"`
	CreatedAt      time.Time `json:"createdAt" bson:"createdAt"`
	Sender         string    `json:"sender" bson:"sender"`
	IsAttachment   bool      `json:"isAttachment" bson:"isAttachment"`
	AttachmentType string    `json:"attachmentType" bson:"attachmentType"`
	AttachmentURL  string    `json:"attachmentURL" bson:"attachmentURL"`
}
