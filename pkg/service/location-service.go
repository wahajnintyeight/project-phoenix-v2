package service

import (
	"log"
	"project-phoenix/v2/internal/broker"
	internal "project-phoenix/v2/internal/service-configs"
	"sync"
	microBroker "go-micro.dev/v4/broker"

	"go-micro.dev/v4"
)

type LocationService struct {
	service          micro.Service
	subscribedTopics []internal.SubscribedTopicsMap
	brokerObj        microBroker.Broker
}

// var locationServiceObj *LocationService

var locationOnce sync.Once

func (ls *LocationService) GetSubscribedTopics() []internal.SubscribedTopicsMap {
	serviceConfig, e := internal.ReturnServiceConfig("location-service")
	if e != nil {
		log.Println("Unable to read service config", e)
		return nil
	}
	ls.subscribedTopics = serviceConfig.(*internal.ServiceConfig).SubscribedTopics
	return ls.subscribedTopics
}

func (ls *LocationService) SubscribeTopics() {
	for _, topic := range ls.subscribedTopics {
		ls.brokerObj.Subscribe(topic.TopicName,ls.ListenSubscribedTopics,microBroker.SubscribeOptions.Queue(topic.TopicName))
	}
}

func (ls *LocationService) ListenSubscribedTopics(broker broker.Event) {
	// ls.brokerObj.Subscribe()
	// broker
}

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
