package service

import (
	// "context"

	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"sync"
	"time"

	"project-phoenix/v2/internal/model"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/handler"
	"project-phoenix/v2/pkg/helper"
	"project-phoenix/v2/pkg/service"

	// "sync"

	"github.com/go-micro/plugins/v4/broker/rabbitmq"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go-micro.dev/v4"
	microBroker "go-micro.dev/v4/broker"
	// "log"
)

type SSEService struct {
	service            micro.Service
	router             *mux.Router
	server             *http.Server
	serviceConfig      internal.ServiceConfig
	subscribedServices []internal.SubscribedServices
	brokerObj          microBroker.Broker
	EventChannel chan string
	IsDisconnected chan bool
	clients    map[chan string]bool
	addClient  chan chan string
	removeClient chan chan string
	broadcast  chan map[string]interface{}
}

var once sync.Once

const (
	MaxRetries  = 6
	RetryDelay  = 2 * time.Second
	serviceName = "sse-service"
)

func (sse *SSEService) GetSubscribedTopics() []internal.SubscribedServices {
	serviceConfig, e := internal.ReturnServiceConfig(serviceName)
	if e != nil {
		log.Println("Unable to read service config", e)
		return nil
	}
	sse.subscribedServices = serviceConfig.(*internal.ServiceConfig).SubscribedServices
	log.Println("SSE Service Subscribed Services: ", sse.subscribedServices)
	return sse.subscribedServices
}

func (sse *SSEService) InitializeService(serviceObj micro.Service, serviceName string) service.ServiceInterface {

	once.Do(func() {
		service := serviceObj
		sse.service = service
		sse.router = mux.NewRouter()
		sse.brokerObj = service.Options().Broker
		godotenv.Load()
		servicePath := "sse-service"
		serviceConfig, _ := internal.ReturnServiceConfig(servicePath)
		sse.serviceConfig = serviceConfig.(internal.ServiceConfig)
		log.Println("sse Service Config", sse.serviceConfig)
		log.Println("sse Gateway Service Broker Instance: ", sse.brokerObj)

		sse.clients = make(map[chan string]bool)
		sse.addClient = make(chan chan string)
		sse.removeClient = make(chan chan string)
		sse.broadcast = make(chan map[string]interface{})
	})

	return sse
}

func (sse *SSEService) ListenSubscribedTopics(broker microBroker.Event) error {
	// ls.brokerObj.Subscribe()
	// broker
	log.Println("Broker Event: ", broker)
	log.Println("Broker Event: ", broker.Message().Header)
	return nil
}

func (ls *SSEService) SubscribeTopics() {
	ls.InitServiceConfig()
	for _, service := range ls.serviceConfig.SubscribedServices {
		log.Println("Service", service)
		for _, topic := range service.SubscribedTopics {
			log.Println("Preparing to subscribe to service: ", service.Name, " | Topic: ", topic.TopicName, " | Queue: ", service.Queue, " | Handler: ", topic.TopicHandler, " | MaxRetries: ", MaxRetries)
			if err := ls.attemptSubscribe(service.Queue, topic); err != nil {
				log.Printf("Subscription failed for topic %s: %v", topic.TopicName, err)
			}
		}
	}
}

// attemptSubscribe tries to subscribe to a topic with retries until successful or max retries reached.
func (sse *SSEService) attemptSubscribe(queueName string, topic internal.SubscribedTopicsMap) error {
	log.Println("Max Retries:", MaxRetries)
	for i := 0; i <= MaxRetries; i++ {
		log.Println("Attempting to subscribe to", topic, " for Queue", queueName)
		if err := sse.subscribeToTopic(queueName, topic); err != nil {
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


func (sse *SSEService) subscribeToTopic(queueName string, topic internal.SubscribedTopicsMap) error {
	handler, ok := reflect.TypeOf(sse).MethodByName(topic.TopicHandler)
	if !ok {
		return fmt.Errorf("Handler method %s not found for topic %s", topic.TopicHandler, topic.TopicName)
	}

	_, err := sse.brokerObj.Subscribe(topic.TopicName, func(p microBroker.Event) error {
		returnValues := handler.Func.Call([]reflect.Value{reflect.ValueOf(sse), reflect.ValueOf(p)})
		if err, ok := returnValues[0].Interface().(error); ok && err != nil {
			return err
		}
		return nil
	}, microBroker.Queue(queueName), rabbitmq.DurableQueue())

	if err != nil {
		log.Printf("Failed to subscribe to topic %s due to error: %v", topic.TopicName, err)
		return err
	}

	log.Printf("Successfully subscribed to topic %s | Handler: %s", topic.TopicName, topic.TopicHandler)
	return nil
}

func (sse *SSEService) HandleCaptureDeviceData(p microBroker.Event) error {
	log.Println("Process Capture Device Data Func Called | Data: ")

	data := make(map[string]interface{})
	if err := json.Unmarshal(p.Message().Body, &data); err != nil {
		log.Println("Error occurred while unmarshalling the data", err)
		return err
	}
	deviceData := model.Device{}
	er := helper.InterfaceToStruct(data["data"], &deviceData)
	if er != nil {
		log.Println("Error decoding the data map", er)
		return er
	}
	log.Println("Data Received: ", deviceData)

	deviceDataMap, err := helper.StructToMap(deviceData)
	if err != nil {
		log.Println("Error occurred while converting the device data to map", err)
		return err
	}
	sse.broadcast <- deviceDataMap
	return nil
}

func (ls *SSEService) InitServiceConfig() {
	serviceConfig, er := internal.ReturnServiceConfig("sse-service")
	if er != nil {
		log.Println("Unable to read service config", er)
		return
	}
	ls.serviceConfig = serviceConfig.(internal.ServiceConfig)
}

func NewSSEService(serviceObj micro.Service, serviceName string) service.ServiceInterface {
	SSEService := &SSEService{}
	return SSEService.InitializeService(serviceObj, serviceName)
}

func (s *SSEService) registerRoutes() {
	
}
 

func (sse *SSEService) Start(port string) error {
	godotenv.Load()

	log.Println("Starting sse Gateway Service on Port:", port)

	sse.SubscribeTopics()
	
	ssrHandler := handler.NewSSERequestHandler()
	go ssrHandler.Run()
	

	http.Handle("/events", ssrHandler)

	sse.server = &http.Server{
		Addr:    ":" + port,
		Handler: sse.router,
	}

	if err := sse.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server ListenAndServe error: %v", err)
	}

	return nil
}

func (sse *SSEService) Stop() error {
	// Stop the sse Gateway service
	log.Println("Stopping sse Gateway")
	return nil
}
