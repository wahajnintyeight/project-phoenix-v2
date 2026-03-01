package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"sync"
	"time"

	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/service"
	"project-phoenix/v2/pkg/service/worker-service/handlers"

	"github.com/go-micro/plugins/v4/broker/rabbitmq"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go-micro.dev/v4"
	microBroker "go-micro.dev/v4/broker"
)

type WorkerService struct {
	service            micro.Service
	router             *mux.Router
	server             *http.Server
	serviceConfig      internal.ServiceConfig
	subscribedServices []internal.SubscribedServices
	brokerObj          microBroker.Broker
	screenshotHandler  *handlers.ScreenshotHandler
}

var once sync.Once

const (
	MaxRetries  = 6
	RetryDelay  = 2 * time.Second
	serviceName = "worker-service"
)

func (qc *WorkerService) GetSubscribedTopics() []internal.SubscribedServices {
	serviceConfig, e := internal.ReturnServiceConfig(serviceName)
	if e != nil {
		log.Println("Unable to read service config", e)
		return nil
	}
	qc.subscribedServices = serviceConfig.(*internal.ServiceConfig).SubscribedServices
	log.Println("Worker Service Subscribed Services: ", qc.subscribedServices)
	return qc.subscribedServices
}

func (qc *WorkerService) InitializeService(serviceObj micro.Service, serviceName string) service.ServiceInterface {
	once.Do(func() {
		service := serviceObj
		qc.service = service
		qc.router = mux.NewRouter()
		qc.brokerObj = service.Options().Broker
		godotenv.Load()
		servicePath := "worker-service"
		serviceConfig, _ := internal.ReturnServiceConfig(servicePath)
		qc.serviceConfig = serviceConfig.(internal.ServiceConfig)

		// Initialize handlers
		qc.screenshotHandler = handlers.NewScreenshotHandler()

		log.Println("Worker Service initialized")
	})

	return qc
}

func (qc *WorkerService) ListenSubscribedTopics(broker microBroker.Event) error {
	log.Println("Broker Event: ", broker)
	log.Println("Broker Event: ", broker.Message().Header)
	return nil
}

func (qc *WorkerService) SubscribeTopics() {
	qc.InitServiceConfig()
	for _, service := range qc.serviceConfig.SubscribedServices {
		log.Println("Service", service)
		for _, topic := range service.SubscribedTopics {
			log.Println("Preparing to subscribe to service: ", service.Name, " | Topic: ", topic.TopicName, " | Queue: ", service.Queue, " | Handler: ", topic.TopicHandler, " | MaxRetries: ", MaxRetries)
			if err := qc.attemptSubscribe(service.Queue, topic); err != nil {
				log.Printf("Subscription failed for topic %s: %v", topic.TopicName, err)
			}
		}
	}
}

func (qc *WorkerService) attemptSubscribe(queueName string, topic internal.SubscribedTopicsMap) error {
	log.Println("Max Retries:", MaxRetries)
	for i := 0; i <= MaxRetries; i++ {
		log.Println("Attempting to subscribe to", topic, " for Queue", queueName)
		if err := qc.subscribeToTopic(queueName, topic); err != nil {
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

func (qc *WorkerService) subscribeToTopic(queueName string, topic internal.SubscribedTopicsMap) error {
	handler, ok := reflect.TypeOf(qc).MethodByName(topic.TopicHandler)
	if !ok {
		return fmt.Errorf("Handler method %s not found for topic %s", topic.TopicHandler, topic.TopicName)
	}

	_, err := qc.brokerObj.Subscribe(topic.TopicName, func(p microBroker.Event) error {
		returnValues := handler.Func.Call([]reflect.Value{reflect.ValueOf(qc), reflect.ValueOf(p)})
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

// HandleScreenshotProcessing processes screenshot upload events
func (qc *WorkerService) HandleScreenshotProcessing(p microBroker.Event) error {
	log.Println("HandleScreenshotProcessing called")

	data := make(map[string]interface{})
	if err := json.Unmarshal(p.Message().Body, &data); err != nil {
		return fmt.Errorf("error unmarshalling screenshot data: %v", err)
	}

	// Delegate to screenshot handler
	return qc.screenshotHandler.Process(data)
}

func (qc *WorkerService) InitServiceConfig() {
	serviceConfig, er := internal.ReturnServiceConfig("worker-service")
	if er != nil {
		log.Println("Unable to read service config", er)
		return
	}
	qc.serviceConfig = serviceConfig.(internal.ServiceConfig)
}

func NewWorkerService(serviceObj micro.Service, serviceName string) service.ServiceInterface {
	base := (&WorkerService{}).InitializeService(serviceObj, serviceName)
	qc := base.(*WorkerService)
	return qc
}

func (qc *WorkerService) registerRoutes() {
	// Health check endpoint
	qc.router.HandleFunc("/health", qc.handleHealthCheck).Methods("GET")

	// Metrics endpoint (optional)
	qc.router.HandleFunc("/metrics", qc.handleMetrics).Methods("GET")
}

func (qc *WorkerService) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"service": "worker-service",
		"time":    time.Now().UTC(),
	})
}

func (qc *WorkerService) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	metrics := map[string]interface{}{
		"service":         "worker-service",
		"uptime":          time.Since(time.Now()).String(), // TODO: Track actual uptime
		"processed_tasks": 0,                               // TODO: Add counter
		"failed_tasks":    0,                               // TODO: Add counter
		"active_handlers": 1,                               // Screenshot handler
	}

	json.NewEncoder(w).Encode(metrics)
}

func (qc *WorkerService) Start(port string) error {
	godotenv.Load()
	log.Println("Starting Worker Consumer Service on Port:", port)

	qc.SubscribeTopics()

	// Register HTTP routes for health checks
	qc.router = mux.NewRouter()
	qc.registerRoutes()

	qc.server = &http.Server{
		Addr:    ":" + port,
		Handler: qc.router,
	}

	log.Println("Worker Service ready to process tasks")

	if err := qc.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %v", err)
	}

	return nil
}

func (qc *WorkerService) Stop() error {
	log.Println("Stopping Worker Service")
	if qc.server != nil {
		return qc.server.Close()
	}
	return nil
}
