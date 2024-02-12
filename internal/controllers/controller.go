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
	// CreateSession(w http.ResponseWriter, r *http.Request)
}

var (
	controllerRegistry = make(map[string]Controller)
	registryMutex      = sync.Mutex{}
)

func getControllerKey(controllerType enum.ControllerType, dbType enum.DBType, collectionName string) string {
	return fmt.Sprintf("%s-%s-%s", controllerType, dbType, collectionName)
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

func GetControllerInstance(controllerType enum.ControllerType, dbType enum.DBType, collectionName string) Controller {

	key := getControllerKey(controllerType, dbType, collectionName)

	if instance, exists := getRegisteredControllerInstance(key); exists {
		return instance
	}

	var instance Controller
	switch controllerType {
	case enum.SessionController:
		var sessionControllerInstance *SessionController
		once.Do(func() {
			dbInstance, err := db.GetDBInstance(dbType)
			log.Println("DB Instance: ", dbInstance, err)
			if err != nil {
				log.Println("Error while getting DB Instance: ", err)
				return
			}
			sessionControllerInstance = &SessionController{
				DB:             dbInstance,
				CollectionName: collectionName,
			}
			log.Println("Session Controller Instance: ", sessionControllerInstance)
		})
		return sessionControllerInstance
		// add more controllers here
	default:
		log.Println("Unknown controller type: ", controllerType)
		return nil
	}
	registerControllerInstance(key, instance)
	return instance
}
