package controllers

import (
	"encoding/json"
	"log"
	"math/rand/v2"
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
	_, e := cs.DB.Update(map[string]interface{}{"code":roomRequestBody.Code}, updateData, cs.GetCollectionName())
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
		"rooms":    rooms,
	}, nil
}

func (cs *ClipboardRoomController) CreateRoom(w http.ResponseWriter, r *http.Request) (int,interface{}, error) {

	randomCode := generateCode()

	// Get IP and User-Agent from request
	ip := r.RemoteAddr
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		ip = forwardedFor
	}
	userAgent := r.Header.Get("User-Agent")


	roomModelObj := model.ClipboardRoom{
		Code: randomCode,
		CreatedAt: time.Now(),
		RoomName: "Untitled " + time.Now().String(),
		Members: []model.ClipboardRoomMember{
			{
				IP: ip,
				UserAgent: userAgent,
				JoinedAt: time.Now(),
			},
		},
	}

	_,e := cs.Create(roomModelObj)

	if e != nil {
		return int(enum.ROOM_NOT_CREATED), nil, e
	}

	i, e := cs.DB.FindOne(map[string]interface{}{"code":randomCode},cs.GetCollectionName())
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
				"ip": ip,
				"userAgent": userAgent,
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
		code[i] = charset[rand.IntN(len(charset))]
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
