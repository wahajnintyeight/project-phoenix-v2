package controllers

import (
	"encoding/json"
	"github.com/gosimple/slug"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"net/http"
	"project-phoenix/v2/internal/cache"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/pkg/helper"
	"time"
)

type CaptureScreenController struct {
	CollectionName string
	DB             db.DBInterface
}

func (cs *CaptureScreenController) GetCollectionName() string {
	return "devices"
}

func (cs *CaptureScreenController) PerformIndexing() error {
	indexes := []interface{}{"deviceName"}
	var validateErr error
	for _, index := range indexes {
		validateErr = cs.DB.ValidateIndexing(cs.GetCollectionName(), index)
		if validateErr != nil {
			return validateErr
		}
	}
	return nil

}

func (cs *CaptureScreenController) Create(device model.Device) (bson.M, error) {
	d, e := cs.DB.Create(device, cs.GetCollectionName())
	if e != nil {
		log.Println("Error occurred while creating the device", e)
		return nil, e
	}
	return d, nil
}

func (cs *CaptureScreenController) Find(query map[string]interface{}) (bson.M, error) {
	log.Println("Find Device", query)
	d, e := cs.DB.FindOne(query, cs.GetCollectionName())
	if e != nil {
		log.Println("Error occurred while finding the device", e)
		return nil, e
	}
	log.Println("Found", d)
	return d, nil
}


func (cs *CaptureScreenController) Update(query map[string]interface{}, updateData map[string]interface{}) error {
	_, e := cs.DB.Update(query, updateData, cs.GetCollectionName())
	if e != nil {
		log.Println("Error occurred while updating the device", e)
		return e
	}
	return nil
}

func (cs *CaptureScreenController) ListDevices(page int) (int, map[string]interface{}, error) {
	log.Println("List Devices")
	totalPages, page, devices, e := cs.DB.FindAllWithPagination(map[string]interface{}{}, 1, cs.GetCollectionName())
	if e != nil {
		log.Println("Error getting devices", e)
		return int(enum.ERROR), nil, e
	}
	if devices == nil {
		devices = []primitive.M{}
	}
	return int(enum.DEVICES_FOUND), map[string]interface{}{
		"totalPages": totalPages,
		"page":       page,
		"devices":    devices,
	}, nil
}

func (cs *CaptureScreenController) CaptureScreen(w http.ResponseWriter, r *http.Request) (int, error) {

	deviceModel := model.Device{}
	decodeErr := json.NewDecoder(r.Body).Decode(&deviceModel)

	if decodeErr != nil {
		return int(enum.CAPTURE_SCREEN_EVENT_FAILED), decodeErr
	}
	data := "capture-screen-" + slug.Make(deviceModel.DeviceName)
	dataInterface, e := helper.StringToInterface(data)
	if e != nil {
		return int(enum.CAPTURE_SCREEN_EVENT_FAILED), e
	}
	channelName := "capture-screen-" + slug.Make(deviceModel.DeviceName)
	log.Println("Channel Name: ", channelName)
	isPub, e := cache.GetInstance().PublishMessage(dataInterface, channelName)
	if e != nil {
		er := cs.TurnDeviceOffline(map[string]interface{}{"deviceName":deviceModel.DeviceName})
		if er != nil {
			return int(enum.CAPTURE_SCREEN_EVENT_FAILED),  er
		}
		return int(enum.CAPTURE_SCREEN_EVENT_FAILED), e
	}
	if isPub == true {
		return int(enum.CAPTURE_SCREEN_EVENT_SENT), nil
	}
	return -1, nil
}

func (cs *CaptureScreenController) ScanDevices(w http.ResponseWriter, r *http.Request) (int, error) {
	data := "scan-devices"
	dataInterface, e := helper.StringToInterface(data)
	if e != nil {
		return int(enum.SCAN_DEVICE_EVENT_FAILED), e
	}
	channelName := "scan-devices"
	isPub, e := cache.GetInstance().PublishMessage(dataInterface, channelName)
	if e != nil {
		return int(enum.SCAN_DEVICE_EVENT_FAILED), e
	}
	if isPub == true {
		return int(enum.SCAN_DEVICE_EVENT_SENT), nil
	}
	return -1, nil
}

func (cs *CaptureScreenController) ReturnDeviceName(w http.ResponseWriter, r *http.Request) (int, error) {
	deviceModel := model.Device{}
	existingDeviceModel := model.Device{}
	decodeErr := json.NewDecoder(r.Body).Decode(&deviceModel)
	if decodeErr != nil {
		log.Println("Error while decoding device model", decodeErr)
		return int(enum.ERROR), decodeErr
	} else {
		log.Println("Device Name: ", deviceModel.DeviceName)
		deviceQuery := map[string]interface{}{
			"deviceName": deviceModel.DeviceName,
		}
		data, e := cs.Find(deviceQuery)
		if e != nil {
			log.Println("Error finding device", e)
			// return int(enum.DATA_NOT_FETCHED), e
		}
		if data != nil {
			log.Println("Device found", data)
			er := helper.MapToStruct(data, &existingDeviceModel)
			if er != nil {
				log.Println("Error converting map to struct", e)
			}
		}
		if deviceModel.DeviceName != existingDeviceModel.DeviceName || data == nil {
			newDevice := model.Device{
				DeviceName: deviceModel.DeviceName,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				IsOnline:   true,
				LastOnline: time.Now(),
			}
			_, err := cs.Create(newDevice)
			if err != nil {
				log.Println("Error creating device", err)
				return int(enum.DEVICE_NOT_CREATED), err
			}
		}
		return int(enum.DATA_FETCHED), nil
	}
}

func (cs *CaptureScreenController) DeleteDevice(w http.ResponseWriter, r *http.Request) (int, error) {

	deviceModel := model.Device{}
	log.Println("Params:", r.Body)
	decodeErr := json.NewDecoder(r.Body).Decode(&deviceModel)
	if decodeErr != nil {
		log.Println("Error while decoding device model", decodeErr)
		return int(enum.ERROR), decodeErr
	}
	log.Println("Device: ", deviceModel)
	objectId, er := primitive.ObjectIDFromHex(deviceModel.ID)
	if er != nil {
		return int(enum.DEVICE_FAILED_TO_DELETE), er
	}
	log.Println("Device ObjectId", objectId)
	_, e := cs.DB.Delete(map[string]interface{}{
		"_id": objectId,
	}, cs.GetCollectionName())

	if e != nil {
		log.Println("Failed to delete device:", e)
		return int(enum.DEVICE_FAILED_TO_DELETE), e
	}
	log.Println("Device has been deleted")
	return int(enum.DEVICE_DELETED), nil
}

func (cs *CaptureScreenController) ShowDeviceInfo(deviceId string) (int, interface{}, error) {

	objectId, er := primitive.ObjectIDFromHex(deviceId)
	if er != nil {
		return int(enum.ERROR), nil, er
	}
	deviceQuery := map[string]interface{}{
		"_id": objectId,
	}
	device, e := cs.Find(deviceQuery)
	if e != nil {
		return int(enum.DEVICE_NOT_FOUND), nil, e
	}
	return int(enum.DEVICE_FOUND), device, nil
}


func (cs *CaptureScreenController) PingDevice(w http.ResponseWriter, r *http.Request) (int, error) {
	
	deviceModel := model.CaptureScreenDeviceQueryModel{}
	log.Println("Params:", r.Body)
	decodeErr := json.NewDecoder(r.Body).Decode(&deviceModel)
	if decodeErr != nil {
		log.Println("Error while decoding device model", decodeErr)
		return int(enum.ERROR),  decodeErr
	}

	eventType := "ping-device-"
	channelName := eventType + slug.Make(deviceModel.DeviceName)
	log.Println("Channel Name: ", channelName)
	data := eventType + slug.Make(deviceModel.DeviceName)
	dataInterface, e := helper.StringToInterface(data)
	if e != nil {
		return int(enum.CAPTURE_SCREEN_EVENT_FAILED),  e
	}
	isPub, e := cache.GetInstance().PublishMessage(dataInterface, channelName)
	if e != nil {
		er := cs.TurnDeviceOffline(map[string]interface{}{"deviceName":deviceModel.DeviceName})
		if er != nil {
			return int(enum.CAPTURE_SCREEN_EVENT_FAILED),  er
		}
		return int(enum.CAPTURE_SCREEN_EVENT_FAILED),  e
	}
	if isPub == true {
		return int(enum.CAPTURE_SCREEN_EVENT_SENT),  nil
	}
	return -1, nil
} 

func (cs *CaptureScreenController) TurnDeviceOffline (query map[string]interface{}) (error){
	updateData := map[string]interface{}{
		"isOnline": false,
		"updatedAt": time.Now().UTC(),
	}
	_, e := cs.DB.Update(query, updateData, cs.GetCollectionName())
	if e != nil {
		log.Println("Error occurred while updating the device", e)
		return e
	}
	return nil
}