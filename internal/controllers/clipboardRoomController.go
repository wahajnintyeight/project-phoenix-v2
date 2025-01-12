package controllers

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/pkg/helper"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ClipboardRoomController struct {
	CollectionName string
	DB             db.DBInterface
}

func (cs *ClipboardRoomController) GetCollectionName() string {
	return "clipboardRooms"
}

func (cs *ClipboardRoomController) PerformIndexing() error {
	indexes := []interface{}{"roomName"}
	var validateErr error
	for _, index := range indexes {
		validateErr = cs.DB.ValidateIndexing(cs.GetCollectionName(), index)
		if validateErr != nil {
			return validateErr
		}
	}
	return nil

}

func (cs *ClipboardRoomController) Create(room model.ClipboardRoom) (bson.M, error) {
	d, e := cs.DB.Create(room, cs.GetCollectionName())
	if e != nil {
		log.Println("Error occurred while creating the device", e)
		return nil, e
	}
	return d, nil
}

func (cs *ClipboardRoomController) Find(query map[string]interface{}) (bson.M, error) {
	log.Println("Find Room", query)
	d, e := cs.DB.FindOne(query, cs.GetCollectionName())
	if e != nil {
		log.Println("Error occurred while finding the room", e)
		return nil, e
	}
	log.Println("Found", d)
	return d, nil
}

func (cs *ClipboardRoomController) Update(w http.ResponseWriter, r *http.Request) error {
	roomRequestBody := model.ClipboardUpdateNameRequestModel{}
	decodeErr := json.NewDecoder(r.Body).Decode(&roomRequestBody)
	if decodeErr != nil {
		return decodeErr
	}
	updateData := map[string]interface{}{
		"roomName": roomRequestBody.RoomName,
	}
	_, e := cs.DB.Update(map[string]interface{}{"code": roomRequestBody.Code}, updateData, cs.GetCollectionName())
	if e != nil {
		log.Println("Error occurred while updating the device", e)
		return e
	}
	return nil
}

func (cs *ClipboardRoomController) ListRooms(page int) (int, map[string]interface{}, error) {
	log.Println("List Rooms")
	totalPages, page, rooms, e := cs.DB.FindAllWithPagination(map[string]interface{}{}, 1, cs.GetCollectionName())
	if e != nil {
		log.Println("Error getting rooms", e)
		return int(enum.ERROR), nil, e
	}
	if rooms == nil {
		rooms = []primitive.M{}
	}
	return int(enum.DEVICES_FOUND), map[string]interface{}{
		"totalPages": totalPages,
		"page":       page,
		"rooms":      rooms,
	}, nil
}

func (cs *ClipboardRoomController) CreateRoom(w http.ResponseWriter, r *http.Request) (int, interface{}, error) {

	randomCode := generateCode()

	// Get IP and User-Agent from request
	ip := r.RemoteAddr
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		ip = forwardedFor
	}
	userAgent := r.Header.Get("User-Agent")

	roomRequestBody := model.ClipboardRequestModel{}
	decodeErr := json.NewDecoder(r.Body).Decode(&roomRequestBody)
	if decodeErr != nil {
		return int(enum.ERROR), nil, decodeErr
	}
	roomModelObj := model.ClipboardRoom{
		Code:      randomCode,
		CreatedAt: time.Now(),
		RoomName:  "Untitled " + time.Now().String(),
		Members: []model.ClipboardRoomMember{
			{
				IP:         ip,
				UserAgent:  userAgent,
				JoinedAt:   time.Now(),
				DeviceInfo: roomRequestBody.DeviceInfo,
			},
		},
		Messages: []model.ClipboardRoomMessage{},
	}

	_, e := cs.Create(roomModelObj)

	if e != nil {
		return int(enum.ROOM_NOT_CREATED), nil, e
	}

	i, e := cs.DB.FindOne(map[string]interface{}{"code": randomCode}, cs.GetCollectionName())
	if e != nil {
		return int(enum.ROOM_NOT_CREATED), nil, nil
	}

	return int(enum.ROOM_CREATED), i, nil

}

func (cs *ClipboardRoomController) JoinRoom(w http.ResponseWriter, r *http.Request) (int, interface{}, error) {
	roomRequestBody := model.ClipboardRequestModel{}
	decodeErr := json.NewDecoder(r.Body).Decode(&roomRequestBody)
	if decodeErr != nil {
		return int(enum.ROOM_NOT_FOUND), nil, decodeErr
	}

	// Get IP and User-Agent from request
	ip := r.RemoteAddr
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		ip = forwardedFor
	}
	userAgent := r.Header.Get("User-Agent")

	// First check if user is already a member
	existingRoom, e := cs.DB.FindOne(map[string]interface{}{
		"code": roomRequestBody.Code,
		"members": map[string]interface{}{
			"$elemMatch": map[string]interface{}{
				"deviceInfo": roomRequestBody.DeviceInfo,
			},
		},
	}, cs.GetCollectionName())

	if existingRoom != nil {
		roomModel := model.ClipboardRoom{}
		if err := helper.MapToStruct(existingRoom, &roomModel); err != nil {
			return int(enum.ERROR), nil, err
		}
		return int(enum.ROOM_FOUND), roomModel, nil
	}

	// If not found as member, get room info
	roomInfo, e := cs.DB.FindOne(map[string]interface{}{"code": roomRequestBody.Code}, cs.GetCollectionName())
	if e != nil {
		return int(enum.ROOM_NOT_FOUND), nil, e
	}

	roomModel := model.ClipboardRoom{}
	er := helper.MapToStruct(roomInfo, &roomModel)
	if er != nil {
		return int(enum.ERROR), nil, er
	}

	// Create new member
	newMember := model.ClipboardRoomMember{
		IP:        ip,
		UserAgent: userAgent,
		JoinedAt:  time.Now(),
		DeviceInfo: roomRequestBody.DeviceInfo,
	}

	roomModel.Members = append(roomModel.Members, newMember)
	_, err := cs.DB.Update(map[string]interface{}{"code": roomRequestBody.Code},
		map[string]interface{}{"members": roomModel.Members},
		cs.GetCollectionName())
	if err != nil {
		return int(enum.ERROR), nil, err
	}

	return int(enum.ROOM_FOUND), roomModel, nil

}

func generateCode() string {
	charset := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, 6)

	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}

	return string(code)
}

func (cs *ClipboardRoomController) DeleteRoom(w http.ResponseWriter, r *http.Request) (int, error) {

	roomModel := model.ClipboardRoom{}
	log.Println("Params:", r.Body)
	decodeErr := json.NewDecoder(r.Body).Decode(&roomModel)
	if decodeErr != nil {
		log.Println("Error while decoding room model", decodeErr)
		return int(enum.ERROR), decodeErr
	}
	log.Println("Room: ", roomModel)
	objectId, er := primitive.ObjectIDFromHex(roomModel.ID)
	if er != nil {
		return int(enum.ROOM_NOT_DELETED), er
	}
	log.Println("Room ObjectId", objectId)
	_, e := cs.DB.Delete(map[string]interface{}{
		"_id": objectId,
	}, cs.GetCollectionName())

	if e != nil {
		log.Println("Failed to delete room:", e)
		return int(enum.ROOM_NOT_DELETED), e
	}
	log.Println("Room has been deleted")
	return int(enum.ROOM_DELETED), nil
}

func (cs *ClipboardRoomController) ShowRoomInfo(roomId string) (int, interface{}, error) {

	objectId, er := primitive.ObjectIDFromHex(roomId)
	if er != nil {
		return int(enum.ERROR), nil, er
	}
	roomQuery := map[string]interface{}{
		"_id": objectId,
	}
	room, e := cs.Find(roomQuery)
	if e != nil {
		return int(enum.ROOM_NOT_FOUND), nil, e
	}
	return int(enum.ROOM_FOUND), room, nil
}

func (cs *ClipboardRoomController) ProcessRoomMessage(roomCode string, data map[string]interface{}) (int, interface{}, error) {
	log.Println("Processing room message:", data)

	// // Extract message data
	messageData := model.ClipBoardSendRoomMessage{}
	err := helper.InterfaceToStruct(data["data"], &messageData)
	if err != nil {
		log.Println("Error parsing message data:", err)
		return int(enum.ERROR), nil, err
	}

	// Create message object
	message := map[string]interface{}{
		"roomId":         messageData.RoomID,
		"message":        messageData.Message,
		"createdAt":      messageData.TimeStamp,
		"sender":         messageData.Sender,
		"isAttachment":   messageData.IsAttachment,
		"attachmentType": messageData.AttachmentType,
		"attachmentURL":  messageData.AttachmentURL,
		"deviceInfo":     messageData.DeviceInfo.SlugifiedDeviceName,
	}

	log.Println("Message:", message)
	// Update room with new message
	updateData := bson.M{"messages": message}
	setData := bson.M{"lastMessage": messageData.TimeStamp}
	incMap := map[string]interface{}{"totalMessages": 1}

	_, err = cs.DB.UpdateAndIncrement(map[string]interface{}{"code": roomCode}, updateData, incMap, setData, cs.GetCollectionName())

	if err != nil {
		log.Println("Error saving message:", err)
		return int(enum.ERROR), nil, err
	}

	return int(enum.ROOM_UPDATED), message, nil
}
