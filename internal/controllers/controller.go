package controllers

import (
	"fmt"
	"log"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"sync"
)

var (
	once sync.Once
)

type Controller interface {
}

var (
	controllerRegistry = make(map[string]Controller)
	registryMutex      = sync.Mutex{}

	// Controllers
	sessionControllerInstance         *SessionController
	userControllerInstance            *UserController
	userTripControllerInstance        *UserTripController
	userLocationControllerInstance    *UserLocationController
	userTripHistoryControllerInstance *UserTripHistoryController
	loginActivityControllerInstance   *LoginActivityController
	captureScreenControllerInstance   *CaptureScreenController
	clipboardRoomControllerInstance   *ClipboardRoomController
	googleControllerInstance   		  *GoogleController
)

func getControllerKey(controllerType enum.ControllerType, dbType enum.DBType) string {
	return fmt.Sprintf("%s-%s", controllerType, dbType)
}

func registerControllerInstance(key string, instance Controller) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	controllerRegistry[key] = instance
}

func getRegisteredControllerInstance(key string) (Controller, bool) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	instance, exists := controllerRegistry[key]
	return instance, exists
}

func GetControllerInstance(controllerType enum.ControllerType, dbType enum.DBType) Controller {

	key := getControllerKey(controllerType, dbType)

	if instance, exists := getRegisteredControllerInstance(key); exists {
		return instance
	}

	switch controllerType {
	case enum.SessionController:

		once.Do(func() {
			dbInstance, err := db.GetDBInstance(dbType)
			log.Println("DB Instance: ", dbInstance, err)
			if err != nil {
				log.Println("Error while getting DB Instance: ", err)
				return
			}
			sessionControllerInstance = &SessionController{
				DB: dbInstance,
			}

			e := sessionControllerInstance.PerformIndexing()
			if e != nil {
				log.Println("Error while indexing: ", e)
			}
		})
		return sessionControllerInstance
	case enum.UserController:
		if userControllerInstance == nil {
			log.Println("Initialize User Controller")
			dbInstance, err := db.GetDBInstance(dbType)
			if err != nil {
				log.Println("Error while getting DB Instance: ", err)
			}
			userControllerInstance = &UserController{
				DB: dbInstance,
			}

		}
		return userControllerInstance
	case enum.UserTripController:
		if userTripControllerInstance == nil {
			log.Println("Initialize User Trip Controller")
			dbInstance, err := db.GetDBInstance(dbType)
			if err != nil {
				log.Println("Error while getting DB Instance: ", err)
				return nil
			}
			userTripControllerInstance = &UserTripController{
				DB: dbInstance,
			}
		}
		return userTripControllerInstance
	case enum.UserLocationController:
		if userLocationControllerInstance == nil {
			log.Println("Initialize User Location Controller")
			dbInstance, err := db.GetDBInstance(dbType)
			if err != nil {
				log.Println("Error while getting DB Instance: ", err)
				return nil
			}
			userLocationControllerInstance = &UserLocationController{
				DB: dbInstance,
			}

			e := userLocationControllerInstance.PerformIndexing()
			if e != nil {
				log.Println("Error while indexing: ", e)
			}
		}
		return userLocationControllerInstance
	case enum.UserTripHistoryController:
		if userTripHistoryControllerInstance == nil {
			log.Println("Initialize User Trip History Controller")
			dbInstance, err := db.GetDBInstance(dbType)
			if err != nil {
				log.Println("Error while getting DB Instance: ", err)
				return nil
			}
			userTripHistoryControllerInstance = &UserTripHistoryController{
				DB: dbInstance,
			}

			e := userTripHistoryControllerInstance.PerformIndexing()
			if e != nil {
				log.Println("Error while indexing: ", e)
			}
		}
		return userTripHistoryControllerInstance
	case enum.LoginActivityController:
		if loginActivityControllerInstance == nil {
			log.Println("Initialize User Login Activity Controller")
			dbInstance, err := db.GetDBInstance(dbType)
			if err != nil {
				log.Println("Error while getting DB Instance: ", err)
				return nil
			}
			loginActivityControllerInstance = &LoginActivityController{
				DB: dbInstance,
			}

			e := loginActivityControllerInstance.PerformIndexing()
			if e != nil {
				log.Println("Error while indexing: ", e)
			}
		}
		return loginActivityControllerInstance
	case enum.CaptureScreenController:
		if captureScreenControllerInstance == nil {
			log.Println("Initialize Capture Screen Controller")
			dbInstance, err := db.GetDBInstance(dbType)
			if err != nil {
				log.Println("Error while getting DB Instance ", err)
				return nil
			}
			captureScreenControllerInstance = &CaptureScreenController{
				DB: dbInstance,
			}
			e := captureScreenControllerInstance.PerformIndexing()
			if e != nil {
				log.Println("error while indexing: ", e)
			}
		}
		return captureScreenControllerInstance
	case enum.ClipboardRoomController:
		if clipboardRoomControllerInstance == nil {
			log.Println("Initialize Clipboard Room Controller")
			dbInstance, err := db.GetDBInstance(dbType)
			if err != nil {
				log.Println("Error while getting DB Instance: ", err)
				return nil
			}
			clipboardRoomControllerInstance = &ClipboardRoomController{
				DB: dbInstance,
			}
		}
		return clipboardRoomControllerInstance
	case enum.GoogleController:
		if googleControllerInstance == nil {
			log.Println("Initialize Google Controller")
			googleControllerInstance = &GoogleController{
				DB: nil,
			}
		}
		return googleControllerInstance
	default:
		log.Println("Unknown controller type: ", controllerType)
		return nil
	}
}
