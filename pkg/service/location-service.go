package service

import (
	"log"
	internal "project-phoenix/v2/internal/service-configs"
	"reflect"
	"sync"

	microBroker "go-micro.dev/v4/broker"

	"go-micro.dev/v4"
)

type LocationService struct {
	service            micro.Service
	subscribedServices []internal.SubscribedServices
	serviceConfig      internal.ServiceConfig
	brokerObj          microBroker.Broker
}

// var locationServiceObj *LocationService

var locationOnce sync.Once

func (ls *LocationService) GetSubscribedTopics() []internal.SubscribedServices {
	serviceConfig, e := internal.ReturnServiceConfig("location-service")
	if e != nil {
		log.Println("Unable to read service config", e)
		return nil
	}
	ls.subscribedServices = serviceConfig.(*internal.ServiceConfig).SubscribedServices
	return ls.subscribedServices
}

func (ls *LocationService) InitServiceConfig() {
	serviceConfig, er := internal.ReturnServiceConfig("location-service")
	if er != nil {
		log.Println("Unable to read service config", er)
		return
	}
	ls.serviceConfig = serviceConfig.(internal.ServiceConfig)
}

func (ls *LocationService) SubscribeTopics() {
	ls.InitServiceConfig()
	log.Println("Broker Instance", ls.brokerObj.String(), &ls.brokerObj)
	for _, service := range ls.serviceConfig.SubscribedServices {
		log.Println("Subscribed Service: ", service.Name)
		// Assuming a method exists on ls to handle the topic appropriately
		subscribedTopic := service.SubscribedTopics
		for _, topic := range subscribedTopic {
			log.Println("Subscribed Topic: ", topic.TopicName, topic.TopicHandler, "| Queue: ", service.Queue)
			if handler, ok := reflect.TypeOf(ls).MethodByName(topic.TopicHandler); ok {
				_, err := ls.brokerObj.Subscribe(topic.TopicName, func(p microBroker.Event) error {
					// Use reflection to call the handler method dynamically
					returnValues := handler.Func.Call([]reflect.Value{reflect.ValueOf(&ls), reflect.ValueOf(p)})
					// Assuming the handler method returns only an error
					if err, ok := returnValues[0].Interface().(error); ok && err != nil {
						return err
					}
					return nil
				}, microBroker.Queue(service.Queue))

				if err != nil {
					log.Printf("Failed to subscribe to topic %s: %v", topic.TopicName, err)
				}
			} else {
				log.Printf("Handler method %s not found for topic %s", topic.TopicHandler, topic.TopicName)
			}
		}
	}
}

func (ls *LocationService) ListenSubscribedTopics(broker microBroker.Event) error {
	// ls.brokerObj.Subscribe()
	// broker
	log.Println("Broker Event: ", broker)
	log.Println("Broker Event: ", broker.Message().Header)
	return nil
}

func (ls *LocationService) InitializeService(serviceObj micro.Service, serviceName string) ServiceInterface {

	locationOnce.Do(func() {
		service := serviceObj
		ls.service = service
		ls.brokerObj = serviceObj.Options().Broker
		// ls.service.Run()
		log.Println("Location Service Broker Instance: ", ls.brokerObj)
	})
	return ls
}

func (ls *LocationService) HandleStartTracking(p microBroker.Event) error {
	log.Println("Start Tracking Func Called")
	return nil
}

func (ls *LocationService) HandleStopTracking(p microBroker.Event) error {
	log.Println("Stop Tracking Func Called")
	return nil
}

func NewLocationService(serviceObj micro.Service, serviceName string) ServiceInterface {
	locationService := &LocationService{}
	return locationService.InitializeService(serviceObj, serviceName)
}

func (ls *LocationService) Start() error {
	log.Print("Location Service Started on Port:", ls.service.Server().Options().Address)
	// ls.service.Init()
	// ls.service.Run()
	ls.SubscribeTopics()
	return nil
}

func (ls *LocationService) Stop() error {
	return nil
}
