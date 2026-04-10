package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"sync"
	"time"

	"project-phoenix/v2/internal/notifier"
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
	cricketHandler     *handlers.CricketHandler
	verifierHandler    *handlers.VerifierHandler
	startTime          time.Time
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

		// Initialize Discord Notifier
		webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
		if webhookURL == "" {
			// Default to the provided URL if not in env
			webhookURL = "https://discord.com/api/webhooks/1478082369977323532/9y-KGcM7JhG4XuM6bFOw_ASga2mVsP4PirFtMc33QY_v1NgOoUkpNcg8VH3ckj2HqiwP"
		}
		discordNotifier := notifier.NewDiscordNotifier(webhookURL)

		// Initialize handlers
		qc.screenshotHandler = handlers.NewScreenshotHandler()
		qc.cricketHandler = handlers.NewCricketHandler(discordNotifier)
		qc.verifierHandler = handlers.NewVerifierHandler()
		qc.startTime = time.Now()

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

	uptime := time.Since(qc.startTime)
	verifierStats := qc.verifierHandler.GetStats()

	metrics := map[string]interface{}{
		"service":         "worker-service",
		"uptime":          uptime.String(),
		"processed_tasks": 0, // TODO: Add counter
		"failed_tasks":    0, // TODO: Add counter
		"active_handlers": 3, // Screenshot, Cricket, Verifier handlers
		"verifier": map[string]interface{}{
			"processed": verifierStats["processed"],
			"valid":     verifierStats["valid"],
			"invalid":   verifierStats["invalid"],
		},
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

	// Start scheduled validation cycle (every hour)
	go qc.startValidationScheduler()

	// Start re-validation scheduler for valid keys (every 5 minutes)
	go qc.startRevalidationScheduler()

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

// HandleCricketEvent processes cricket game events
func (qc *WorkerService) HandleCricketEvent(p microBroker.Event) error {
	log.Println("HandleCricketEvent called")

	data := make(map[string]interface{})
	if err := json.Unmarshal(p.Message().Body, &data); err != nil {
		return fmt.Errorf("error unmarshalling cricket event data: %v", err)
	}

	// Delegate to cricket handler
	return qc.cricketHandler.Process(data)
}

// HandleKeyDiscovered processes key discovery events from scraper service
func (qc *WorkerService) HandleKeyDiscovered(p microBroker.Event) error {
	// log.Println("HandleKeyDiscovered called")

	data := make(map[string]interface{})
	if err := json.Unmarshal(p.Message().Body, &data); err != nil {
		return fmt.Errorf("error unmarshalling key discovery data: %v", err)
	}

	// Delegate to verifier handler
	return qc.verifierHandler.Process(data)
}

// startValidationScheduler starts the scheduled validation cycle
func (qc *WorkerService) startValidationScheduler() {
	// Get validation interval from environment (default: 60 minutes)
	intervalMinutes := 60
	if envInterval := os.Getenv("VALIDATION_INTERVAL_MINUTES"); envInterval != "" {
		if parsed, err := time.ParseDuration(envInterval + "m"); err == nil {
			intervalMinutes = int(parsed.Minutes())
		}
	}

	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	defer ticker.Stop()

	log.Printf("Validation scheduler started (interval: %d minutes)", intervalMinutes)

	// Run immediately on startup
	if err := qc.verifierHandler.RunValidationCycle(qc.brokerObj); err != nil {
		log.Printf("Initial validation cycle error: %v", err)
	}

	// Then run on schedule
	for range ticker.C {
		log.Println("Running scheduled validation cycle...")
		if err := qc.verifierHandler.RunValidationCycle(qc.brokerObj); err != nil {
			log.Printf("Scheduled validation cycle error: %v", err)
		}
	}
}

// startRevalidationScheduler starts the scheduled re-validation cycle for valid keys
func (qc *WorkerService) startRevalidationScheduler() {
	// Get re-validation interval from environment (default: 5 minutes)
	intervalMinutes := 5
	if envInterval := os.Getenv("REVALIDATION_INTERVAL_MINUTES"); envInterval != "" {
		if parsed, err := time.ParseDuration(envInterval + "m"); err == nil {
			intervalMinutes = int(parsed.Minutes())
		}
	}

	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	defer ticker.Stop()

	log.Printf("Re-validation scheduler started (interval: %d minutes)", intervalMinutes)

	// Wait 1 minute before first run to avoid overlap with initial validation
	time.Sleep(1 * time.Minute)

	// Then run on schedule
	for range ticker.C {
		log.Println("Running scheduled re-validation cycle for valid keys...")
		if err := qc.verifierHandler.RunRevalidationCycle(qc.brokerObj); err != nil {
			log.Printf("Scheduled re-validation cycle error: %v", err)
		}
	}
}
