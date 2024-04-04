package service

import (
	// "context"

	"fmt"
	"log"
	"reflect"
	"sync"
	"time"

	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/service"

	// "sync"

	"go-micro.dev/v4"
	microBroker "go-micro.dev/v4/broker"
	// "log"
)

type DataCommunicator struct {
	service            micro.Service
	subscribedServices []internal.SubscribedServices
	serviceConfig      internal.ServiceConfig
	brokerObj          microBroker.Broker
}

const (
	DCMaxRetries  = 5
	DCRetryDelay  = 2 * time.Second
	serviceName = "data-communicator"
)

var dataCommunicatorOnce sync.Once

func (dc *DataCommunicator) GetSubscribedTopics() []internal.SubscribedServices {
	serviceConfig, e := internal.ReturnServiceConfig(serviceName)
	if e != nil {
		log.Println("Unable to read service config", e)
		return nil
	}
	dc.subscribedServices = serviceConfig.(*internal.ServiceConfig).SubscribedServices
	return dc.subscribedServices
}

func (dc *DataCommunicator) InitServiceConfig() {
	serviceConfig, er := internal.ReturnServiceConfig(serviceName)
	if er != nil {
		log.Println("Unable to read service config", er)
		return
	}
	dc.serviceConfig = serviceConfig.(internal.ServiceConfig)
}

func (dc *DataCommunicator) SubscribeTopics() {
	dc.InitServiceConfig()
	for _, service := range dc.serviceConfig.SubscribedServices {
		for _, topic := range service.SubscribedTopics {
			log.Println("Preparing to subscribe to service: ", service.Name, " | Topic: ", topic.TopicName, " | Queue: ", service.Queue, " | Handler: ", topic.TopicHandler)
			if err := dc.attemptSubscribe(service.Queue, topic); err != nil {
				log.Printf("Subscription failed for topic %s: %v", topic.TopicName, err)
			}
		}
	}
}

// attemptSubscribe tries to subscribe to a topic with retries until successful or max retries reached.
func (dc *DataCommunicator) attemptSubscribe(queueName string, topic internal.SubscribedTopicsMap) error {
	for i := 0; i <= DCMaxRetries; i++ {
		if err := dc.subscribeToTopic(queueName, topic); err != nil {
			if err.Error() == "not connected" && i < DCMaxRetries {
				log.Printf("Broker not connected, retrying %d/%d for topic %s", i+1, DCMaxRetries, topic.TopicName)
				time.Sleep(DCRetryDelay)
				continue
			}
			return err
		}
		break
	}
	return nil
}

func (dc *DataCommunicator) subscribeToTopic(queueName string, topic internal.SubscribedTopicsMap) error {
	handler, ok := reflect.TypeOf(dc).MethodByName(topic.TopicHandler)
	if !ok {
		return fmt.Errorf("Handler method %s not found for topic %s", topic.TopicHandler, topic.TopicName)
	}

	_, err := dc.brokerObj.Subscribe(topic.TopicName, func(p microBroker.Event) error {
		returnValues := handler.Func.Call([]reflect.Value{reflect.ValueOf(dc), reflect.ValueOf(p)})
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

func (dc *DataCommunicator) ListenSubscribedTopics(broker microBroker.Event) error {
	// ls.brokerObj.Subscribe()
	// broker
	log.Println("Broker Event: ", broker)
	log.Println("Broker Event: ", broker.Message().Header)
	return nil
}

func (dc *DataCommunicator) InitializeService(serviceObj micro.Service, serviceName string) service.ServiceInterface {

	dataCommunicatorOnce.Do(func() {
		service := serviceObj
		dc.service = service
		dc.brokerObj = serviceObj.Options().Broker
		log.Println("Data Communicator Service Broker Instance: ", dc.brokerObj)
	})
	return dc
}

func NewDataCommunicatorService(serviceObj micro.Service, serviceName string) service.ServiceInterface {
	dataCommunicatorService := &DataCommunicator{}
	return dataCommunicatorService.InitializeService(serviceObj, serviceName)
}

func (dc *DataCommunicator) Start() error {
	log.Print("Location Service Started on Port:", dc.service.Server().Options().Address)
	dc.SubscribeTopics()
	return nil
}

func (dc *DataCommunicator) Stop() error {
	return nil
}
