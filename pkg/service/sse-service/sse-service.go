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

	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/enum"
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
	sseHandler         *handler.SSERequestHandler
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

		// Initialize the SSE handler
		sse.sseHandler = handler.NewSSERequestHandler()
		go sse.sseHandler.Run()
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

	log.Println("Process Capture Device Data Func Called")

	data := make(map[string]interface{})
	if err := json.Unmarshal(p.Message().Body, &data); err != nil {
		return fmt.Errorf("error unmarshalling data: %v", err)
	}

	deviceData := model.Device{}
	messageType := int32(data["messageType"].(float64))
	log.Println("Message Type is", messageType)
	if err := helper.InterfaceToStruct(data["data"], &deviceData); err != nil {
		return fmt.Errorf("error decoding data map: %v", err)
	}

	deviceDataMap, err := helper.StructToMap(deviceData)
	if err != nil {
		return fmt.Errorf("error converting device data to map: %v", err)
	}

	controller := controllers.GetControllerInstance(enum.CaptureScreenController, enum.MONGODB)
	captureScreenController := controller.(*controllers.CaptureScreenController)

	query := map[string]interface{}{
		"deviceName": deviceData.DeviceName,
	}

	updateData := map[string]interface{}{}

	switch messageType {

	case int32(enum.PING_DEVICE):

		log.Println("Attempting to broadcast PING_DEVICE", messageType)
		log.Println("Message Data:", deviceDataMap)
		sse.sseHandler.Broadcast(map[string]interface{}{
			"message": deviceDataMap,
			"type":    "ping_device",
		})

		updateData = map[string]interface{}{
			"isOnline":    true,
			"memoryUsage": deviceData.MemoryUsage,
			"osName":      deviceData.OSName,
			"lastOnline":  time.Now().UTC(),
			"diskUsage":   deviceData.DiskUsage,
		}

		break

	case int32(enum.CAPTURE_SCREEN):

		// Use the SSE handler to broadcast
		log.Println("Attempting to broadcast CAPTURE_SCREEN", messageType)
		log.Println("Message Data:", deviceDataMap)
		sse.sseHandler.Broadcast(map[string]interface{}{
			"message": deviceDataMap,
			"type":    "capture_screen",
		})

		updateData = map[string]interface{}{
			"lastImage":   deviceData.LastImage,
			"isOnline":    true,
			"memoryUsage": deviceData.MemoryUsage,
			"osName":      deviceData.OSName,
			"lastOnline":  time.Now().UTC(),
			"diskUsage":   deviceData.DiskUsage,
		}

		break
	default:
		log.Println("No case found for", messageType)
	}

	e := captureScreenController.Update(query, updateData)
	if e != nil {
		log.Println("Error updating device")
	}
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
	log.Println("Starting SSE Service on Port:", port)

	sse.SubscribeTopics()

	// Create a new router and register the SSE handler
	sse.router = mux.NewRouter()
	sse.router.HandleFunc("/events", sse.sseHandler.ServeHTTP)

	sse.server = &http.Server{
		Addr:    ":" + port,
		Handler: sse.router,
	}

	if err := sse.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %v", err)
	}

	return nil

}

func (sse *SSEService) Stop() error {
	// Stop the sse Gateway service
	log.Println("Stopping sse Gateway")
	return nil
}
