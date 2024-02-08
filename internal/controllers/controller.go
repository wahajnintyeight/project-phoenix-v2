package controllers

import (
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

func GetControllerInstance(controllerType enum.ControllerType) Controller {
	switch controllerType {
	case enum.SessionController:
		once.Do(func() {
			sessionControllerInstance = &SessionController{}
		})
		return sessionControllerInstance
	default:
		panic("Unknown controller type")
	}
}
