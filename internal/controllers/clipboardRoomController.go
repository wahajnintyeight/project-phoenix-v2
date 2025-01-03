package controllers

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"net/http"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
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


func (cs *ClipboardRoomController) Update(query map[string]interface{}, updateData map[string]interface{}) error {
	_, e := cs.DB.Update(query, updateData, cs.GetCollectionName())
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

