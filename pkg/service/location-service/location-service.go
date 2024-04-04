package service

import (
	"encoding/json"
	"fmt"
	"log"
	internal "project-phoenix/v2/internal/service-configs"
	"reflect"
	"sync"
	"time"

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

const (
	MaxRetries  = 5
	RetryDelay  = 2 * time.Second
	serviceName = "location-service"
)

var locationOnce sync.Once

func (ls *LocationService) GetSubscribedTopics() []internal.SubscribedServices {
	serviceConfig, e := internal.ReturnServiceConfig(serviceName)
	if e != nil {
		log.Println("Unable to read service config", e)
		return nil
	}
	ls.subscribedServices = serviceConfig.(*internal.ServiceConfig).SubscribedServices
	return ls.subscribedServices
}

func (ls *LocationService) InitServiceConfig() {
	serviceConfig, er := internal.ReturnServiceConfig(serviceName)
	if er != nil {
		log.Println("Unable to read service config", er)
		return
	}
	ls.serviceConfig = serviceConfig.(internal.ServiceConfig)
}

func (ls *LocationService) SubscribeTopics() {
	ls.InitServiceConfig()
	for _, service := range ls.serviceConfig.SubscribedServices {
		for _, topic := range service.SubscribedTopics {
			log.Println("Preparing to subscribe to service: ", service.Name, " | Topic: ", topic.TopicName, " | Queue: ", service.Queue, " | Handler: ", topic.TopicHandler)
			if err := ls.attemptSubscribe(service.Queue, topic); err != nil {
				log.Printf("Subscription failed for topic %s: %v", topic.TopicName, err)
			}
		}
	}
}

// attemptSubscribe tries to subscribe to a topic with retries until successful or max retries reached.
func (ls *LocationService) attemptSubscribe(queueName string, topic internal.SubscribedTopicsMap) error {
	for i := 0; i <= MaxRetries; i++ {
		if err := ls.subscribeToTopic(queueName, topic); err != nil {
			if err.Error() == "not connected" && i < MaxRetries {
				log.Printf("Broker not connected, retrying %d/%d for topic %s", i+1, MaxRetries, topic.TopicName)
				time.Sleep(RetryDelay)
				continue
			}
			return err
		}
		break
	}
	return nil
}

func (ls *LocationService) subscribeToTopic(queueName string, topic internal.SubscribedTopicsMap) error {
	handler, ok := reflect.TypeOf(ls).MethodByName(topic.TopicHandler)
	if !ok {
		return fmt.Errorf("Handler method %s not found for topic %s", topic.TopicHandler, topic.TopicName)
	}

	_, err := ls.brokerObj.Subscribe(topic.TopicName, func(p microBroker.Event) error {
		returnValues := handler.Func.Call([]reflect.Value{reflect.ValueOf(ls), reflect.ValueOf(p)})
		if err, ok := returnValues[0].Interface().(error); ok && err != nil {
			return err
		}
		return nil
	}, microBroker.Queue(queueName))

	if err != nil {
		log.Printf("Failed to subscribe to topic %s due to error: %v", topic.TopicName, err)
		return err
	}

	log.Printf("Successfully subscribed to topic %s | Handler: %s", topic.TopicName, topic.TopicHandler)
	return nil
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
		log.Println("Location Service Broker Instance: ", ls.brokerObj)
	})
	return ls
}

func (ls *LocationService) HandleStartTracking(p microBroker.Event) error {
	log.Println("Start Tracking Func Called | Data: ", p.Message().Header, " | Body: ", p.Message().Body)
	data := make(map[string]interface{})
	if err := json.Unmarshal(p.Message().Body, &data); err != nil {
		log.Println("Error occurred while unmarshalling the data", err)
	}
	log.Println("Data Received: ", data)
	return nil
}

func (ls *LocationService) HandleStopTracking(p microBroker.Event) error {
	log.Println("Stop Tracking Func Called | Data: ", p.Message().Header, " | Body: ", p.Message().Body)
	data := make(map[string]interface{})
	if err := json.Unmarshal(p.Message().Body, &data); err != nil {
		log.Println("Error occurred while unmarshalling the data", err)
	}
	log.Println("Data Received: ", data)
	return nil
}

func NewLocationService(serviceObj micro.Service, serviceName string) ServiceInterface {
	locationService := &LocationService{}
	return locationService.InitializeService(serviceObj, serviceName)
}

func (ls *LocationService) Start() error {
	log.Print("Location Service Started on Port:", ls.service.Server().Options().Address)
	ls.SubscribeTopics()
	return nil
}

func (ls *LocationService) Stop() error {
	return nil
}
