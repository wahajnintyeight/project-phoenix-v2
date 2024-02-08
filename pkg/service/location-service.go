package service

import (
	"log"
	"sync"

	"go-micro.dev/v4"
)

type LocationService struct {
	service micro.Service
}

// var locationServiceObj *LocationService

var locationOnce sync.Once

func (ls *LocationService) InitializeService(serviceObj micro.Service, serviceName string) ServiceInterface {

	locationOnce.Do(func() {
		service := serviceObj
		ls.service = service
	})
	return ls
}

func NewLocationService(serviceObj micro.Service, serviceName string) ServiceInterface {
	locationService := &LocationService{}
	return locationService.InitializeService(serviceObj, serviceName)
}

func (ls *LocationService) Start() error {
	log.Print("Location Service started")
	ls.service.Init()
	return nil
}

func (ls *LocationService) Stop() error {
	return nil
}
