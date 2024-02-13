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
	sessionControllerInstance *SessionController
	userControllerInstance    *UserController
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
	default:
		log.Println("Unknown controller type: ", controllerType)
		return nil
	}
}
