package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	appconfig "project-phoenix/v2/internal/config"
	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/service"
	"project-phoenix/v2/pkg/service/scraper-service/handlers"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go-micro.dev/v4"
	microBroker "go-micro.dev/v4/broker"
)

type ScraperService struct {
	service            micro.Service
	router             *mux.Router
	server             *http.Server
	serviceConfig      internal.ServiceConfig
	subscribedServices []internal.SubscribedServices
	brokerObj          microBroker.Broker
	scraperHandler     *handlers.ScraperHandler
	startTime          time.Time
}

var once sync.Once

const (
	serviceName = "scraper-service"
)

func (s *ScraperService) GetSubscribedTopics() []internal.SubscribedServices {
	serviceConfig, e := internal.ReturnServiceConfig(serviceName)
	if e != nil {
		log.Println("Unable to read service config", e)
		return nil
	}
	s.subscribedServices = serviceConfig.(*internal.ServiceConfig).SubscribedServices
	log.Println("Scraper Service Subscribed Services: ", s.subscribedServices)
	return s.subscribedServices
}

func (s *ScraperService) InitializeService(serviceObj micro.Service, svcName string) service.ServiceInterface {
	once.Do(func() {
		s.service = serviceObj
		s.router = mux.NewRouter()
		s.brokerObj = serviceObj.Options().Broker
		s.startTime = time.Now()
		godotenv.Load()

		// Load and validate configuration
		cfg, err := appconfig.LoadConfig()
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
			os.Exit(1)
		}

		if err := cfg.Validate(); err != nil {
			log.Fatalf("Configuration validation failed: %v", err)
			os.Exit(1)
		}

		log.Println("Configuration validated successfully")

		servicePath := "scraper-service"
		serviceConfig, _ := internal.ReturnServiceConfig(servicePath)
		s.serviceConfig = serviceConfig.(internal.ServiceConfig)

		// Initialize controllers
		apiKeyController := controllers.GetControllerInstance(enum.APIKeyController, enum.MONGODB).(*controllers.APIKeyController)
		scraperConfigController := controllers.GetControllerInstance(enum.ScraperConfigController, enum.MONGODB).(*controllers.ScraperConfigController)

		// Perform indexing
		if err := apiKeyController.PerformIndexing(); err != nil {
			log.Printf("Warning: Failed to create indexes for APIKeyController: %v", err)
		}
		if err := scraperConfigController.PerformIndexing(); err != nil {
			log.Printf("Warning: Failed to create indexes for ScraperConfigController: %v", err)
		}

		// Seed default queries
		if err := scraperConfigController.SeedDefaultQueries(); err != nil {
			log.Printf("Warning: Failed to seed default queries: %v", err)
		}

		// Initialize scraper handler
		s.scraperHandler = handlers.NewScraperHandler(s.brokerObj)

		log.Println("Scraper Service initialized")
	})

	return s
}

func (s *ScraperService) ListenSubscribedTopics(broker microBroker.Event) error {
	log.Println("Broker Event: ", broker)
	log.Println("Broker Event: ", broker.Message().Header)
	return nil
}

func (s *ScraperService) SubscribeTopics() {
	s.InitServiceConfig()
	// Scraper service doesn't subscribe to topics, it only publishes
	log.Println("Scraper Service does not subscribe to any topics")
}

func (s *ScraperService) InitServiceConfig() {
	serviceConfig, er := internal.ReturnServiceConfig(serviceName)
	if er != nil {
		log.Println("Unable to read service config", er)
		return
	}
	s.serviceConfig = serviceConfig.(internal.ServiceConfig)
}

func NewScraperService(serviceObj micro.Service, svcName string) service.ServiceInterface {
	base := (&ScraperService{}).InitializeService(serviceObj, svcName)
	s := base.(*ScraperService)
	return s
}

func (s *ScraperService) registerRoutes() {
	// Health check endpoint
	s.router.HandleFunc("/health", s.handleHealthCheck).Methods("GET")

	// Metrics endpoint
	s.router.HandleFunc("/metrics", s.handleMetrics).Methods("GET")
}

func (s *ScraperService) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check MongoDB connection by trying to get a controller
	mongoStatus := "connected"
	_, err := (&db.MongoDB{}).GetInstance()
	if err != nil {
		mongoStatus = "disconnected"
	}

	// Check RabbitMQ connection
	rabbitmqStatus := "connected"
	if s.brokerObj == nil {
		rabbitmqStatus = "disconnected"
	}

	status := "healthy"
	statusCode := http.StatusOK
	if mongoStatus == "disconnected" || rabbitmqStatus == "disconnected" {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   status,
		"service":  serviceName,
		"time":     time.Now().UTC(),
		"mongodb":  mongoStatus,
		"rabbitmq": rabbitmqStatus,
	})
}

func (s *ScraperService) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	stats := s.scraperHandler.GetStats()
	uptime := time.Since(s.startTime)

	metrics := map[string]interface{}{
		"service":          serviceName,
		"uptime":           uptime.String(),
		"scraping_cycles":  stats["scraping_cycles"],
		"keys_discovered":  stats["keys_discovered"],
		"duplicates_found": stats["duplicates_found"],
		"errors":           stats["errors"],
		"github_rate_limit": map[string]interface{}{
			"remaining": stats["rate_limit_remaining"],
			"reset_at":  stats["rate_limit_reset"],
		},
		"last_scrape": stats["last_scrape"],
	}

	json.NewEncoder(w).Encode(metrics)
}

func (s *ScraperService) Start(port string) error {
	godotenv.Load()
	log.Println("Starting Scraper Service on Port:", port)

	// Register HTTP routes
	s.registerRoutes()

	s.server = &http.Server{
		Addr:    ":" + port,
		Handler: s.router,
	}

	// Start scheduled scraping
	scrapingInterval := os.Getenv("SCRAPING_INTERVAL_MINUTES")
	if scrapingInterval == "" {
		scrapingInterval = "20" // Default to 20 minutes
	}

	go s.startScheduledScraping(scrapingInterval)

	log.Println("Scraper Service ready")

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %v", err)
	}

	return nil
}

func (s *ScraperService) startScheduledScraping(intervalMinutes string) {
	// Parse interval
	var interval time.Duration
	if _, err := fmt.Sscanf(intervalMinutes, "%d", &interval); err != nil {
		log.Printf("Invalid scraping interval, using default 20 minutes: %v", err)
		interval = 20
	}
	interval = interval * time.Minute

	log.Printf("Starting scheduled scraping every %v", interval)

	// Run immediately on startup
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic in initial scraping cycle: %v", r)
			}
		}()
		if err := s.scraperHandler.RunScrapingCycle(); err != nil {
			log.Printf("Error in initial scraping cycle: %v", err)
		}
	}()

	// Then run on schedule
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		log.Printf("Ticker fired at %v - Starting scheduled scraping cycle", time.Now().Format("2006/01/02 15:04:05"))

		// Run each cycle in a separate goroutine with panic recovery
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from panic in scraping cycle: %v", r)
				}
			}()

			if err := s.scraperHandler.RunScrapingCycle(); err != nil {
				log.Printf("Error in scraping cycle: %v", err)
			}
			log.Printf("Scraping cycle completed at %v", time.Now().Format("2006/01/02 15:04:05"))
		}()
	}
}

func (s *ScraperService) Stop() error {
	log.Println("Stopping Scraper Service")
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}
