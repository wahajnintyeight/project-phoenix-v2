package controllers

import (
	"log"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"sync"
)

var (
	once                      sync.Once
	sessionControllerInstance *SessionController
)

type Controller interface {
	// CreateSession(w http.ResponseWriter, r *http.Request)
}

func GetControllerInstance(controllerType enum.ControllerType, dbType enum.DBType, collectionName string) Controller {
	switch controllerType {
	case enum.SessionController:
		once.Do(func() {
			dbInstance, err := db.GetDBInstance(dbType)
			if err != nil {
				panic(err)
			}
			sessionControllerInstance = &SessionController{
				DB:             dbInstance,
				CollectionName: collectionName,
			}
		})
		return sessionControllerInstance
		// add more controllers here
	default:
		log.Panic("Unknown controller type")
		return nil
	}
}
