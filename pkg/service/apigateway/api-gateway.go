package service

import (
	// "context"

	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"project-phoenix/v2/internal/controllers/middleware"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/handler"
	"project-phoenix/v2/pkg/service"

	// "sync"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go-micro.dev/v4"
	microBroker "go-micro.dev/v4/broker"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	// "log"
)

type APIGatewayService struct {
	service            micro.Service
	router             *mux.Router
	server             *http.Server
	serviceConfig      internal.ServiceConfig
	subscribedServices []internal.SubscribedServices
	brokerObj          microBroker.Broker
}

var once sync.Once

func (api *APIGatewayService) GetSubscribedTopics() []internal.SubscribedServices {
	return nil
}

func (api *APIGatewayService) InitializeService(serviceObj micro.Service, serviceName string) service.ServiceInterface {

	once.Do(func() {
		service := serviceObj
		api.service = service
		api.router = mux.NewRouter()
		api.brokerObj = service.Options().Broker
		godotenv.Load()
		servicePath := "api-gateway"
		serviceConfig, _ := internal.ReturnServiceConfig(servicePath)
		api.serviceConfig = serviceConfig.(internal.ServiceConfig)
		log.Println("API Service Config", api.serviceConfig)
		log.Println("API Gateway Service Broker Instance: ", api.brokerObj)
	})

	return api
}

func (api *APIGatewayService) ListenSubscribedTopics(broker microBroker.Event) error {
	// ls.brokerObj.Subscribe()
	// broker
	log.Println("Broker Event: ", broker)
	log.Println("Broker Event: ", broker.Message().Header)
	return nil
}

func (ls *APIGatewayService) SubscribeTopics() {
	ls.InitServiceConfig()
	// for _, topic := range ls.subscribedTopics {
	// 	ls.brokerObj.Subscribe(topic.TopicName, ls.ListenSubscribedTopics, microBroker.Queue(ls.serviceConfig.ServiceQueue))
	// }
}

func (ls *APIGatewayService) InitServiceConfig() {
	serviceConfig, er := internal.ReturnServiceConfig("api-gateway")
	if er != nil {
		log.Println("Unable to read service config", er)
		return
	}
	ls.serviceConfig = serviceConfig.(internal.ServiceConfig)
}

func NewAPIGatewayService(serviceObj micro.Service, serviceName string) service.ServiceInterface {
	apiGatewayService := &APIGatewayService{}
	return apiGatewayService.InitializeService(serviceObj, serviceName)
}

func (s *APIGatewayService) registerRoutes() {
	sessionMiddleware := middleware.NewSessionMiddleware()
	authMiddleware := middleware.NewAuthMiddleware()
	apiRequestHandler := &handler.APIRequestHandler{}
	apiRequestHandler.Endpoint = s.serviceConfig.EndpointPrefix

	openRoutes := []string{
		s.serviceConfig.EndpointPrefix + "/createSession",
		s.serviceConfig.EndpointPrefix + "/",
		s.serviceConfig.EndpointPrefix + "/signJWT",
		s.serviceConfig.EndpointPrefix + "/returnJWK",
		s.serviceConfig.EndpointPrefix + "/handle-webhook",
		s.serviceConfig.EndpointPrefix + "/return-device-name",
		s.serviceConfig.EndpointPrefix + "/search-yt-videos",
		s.serviceConfig.EndpointPrefix + "/download-yt-videos",
		s.serviceConfig.EndpointPrefix + "/llm-api-configs",
		s.serviceConfig.EndpointPrefix + "/llm-api-config",
		s.serviceConfig.EndpointPrefix + "/gollm/test-connection",
		s.serviceConfig.EndpointPrefix + "/gollm/fetch-models",
		s.serviceConfig.EndpointPrefix + "/gollm/ats/scan",
		s.serviceConfig.EndpointPrefix + "/gollm/ats/analyze-resume",
		s.serviceConfig.EndpointPrefix + "/gollm/ats/enhance-description",
		s.serviceConfig.EndpointPrefix + "/gollm/ats/regenerate-item",
		s.serviceConfig.EndpointPrefix + "/gollm/ats/regenerate-skills",
	}
	customMiddlewareWrapper := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the request path is one of the open routes
			log.Println("Request URL: ", r.URL.Path)
			for _, path := range openRoutes {
				if strings.HasSuffix(r.URL.Path, path) {
					apiRequestHandler.ServeHTTP(w, r)
					// next.ServeHTTP(w, r)
					return
				}
			}
			// authMiddleware.Middleware(next).ServeHTTP(w, r)
			sessionMiddleware.Middleware(next).ServeHTTP(w, r)
		})
	}

	openRoutesWithSession := []string{
		s.serviceConfig.EndpointPrefix + "/login",
		s.serviceConfig.EndpointPrefix + "/googleLogin",
		s.serviceConfig.EndpointPrefix + "/visits",
		s.serviceConfig.EndpointPrefix + "/capture-screen",
		s.serviceConfig.EndpointPrefix + "/scan-devices",
		s.serviceConfig.EndpointPrefix + "/ping",
		s.serviceConfig.EndpointPrefix + "/devices",
		s.serviceConfig.EndpointPrefix + "/device", //deleting a device
		s.serviceConfig.EndpointPrefix + "/room/create",
		s.serviceConfig.EndpointPrefix + "/room/join",
		s.serviceConfig.EndpointPrefix + "/room/update",
		s.serviceConfig.EndpointPrefix + "/room/messages",

		s.serviceConfig.EndpointPrefix + "/keys",
		s.serviceConfig.EndpointPrefix + "/stats",
		s.serviceConfig.EndpointPrefix + "/validate-key",
		s.serviceConfig.EndpointPrefix + "/validate-key/openrouter-models",
	}
	customMiddlewareWrapperWithSession := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the request path is one of the open routes
			log.Println("Request URL: ", r.URL.Path)

			for _, path := range openRoutesWithSession {
				if strings.HasSuffix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			sessionMiddleware.Middleware(authMiddleware.Middleware(next)).ServeHTTP(w, r)
		})
	}

	// Custom CORS middleware to handle browser extensions and job sites
	customCORSMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Allow specific origins
			allowedOrigins := map[string]bool{
				"http://localhost:5173":     true,
				"http://localhost:8081":     true,
				"electron://altair":         true,
				"https://www.linkedin.com":  true,
				"https://linkedin.com":      true,
				"https://www.indeed.com":    true,
				"https://indeed.com":        true,
				"https://www.glassdoor.com": true,
				"https://glassdoor.com":     true,
				"https://www.wellfound.com": true,
				"https://wellfound.com":     true,
			}

			isChromeExtension := strings.HasPrefix(origin, "chrome-extension://") || strings.HasPrefix(origin, "moz-extension://")
			isAllowedOrigin := allowedOrigins[origin]

			// Set CORS headers if origin is allowed or is a browser extension
			if isAllowedOrigin || isChromeExtension {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			} else if origin != "" {
				// Allow any other origin for development or if origin is present
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, sessionId, project-type")
			w.Header().Set("Access-Control-Max-Age", "3600")

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}

	// Apply custom CORS middleware to the router
	s.router.Use(customCORSMiddleware)

	s.router.Use(s.ConfigureSentry().Handle)
	s.router.Use(customMiddlewareWrapper)
	s.router.Use(customMiddlewareWrapperWithSession)
	s.router.PathPrefix(s.serviceConfig.EndpointPrefix).Handler(apiRequestHandler)
}

func (s *APIGatewayService) ConfigureSentry() *sentryhttp.Handler {

	sentryDsn := os.Getenv("SENTRY_DSN")
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              sentryDsn,
		TracesSampleRate: 1.0,
		EnableTracing:    true,
		Debug:            true,
	}); err != nil {
		fmt.Printf("Sentry initialization failed: %v\n", err)
	}

	sentryHandler := sentryhttp.New(sentryhttp.Options{
		Repanic:         true,
		WaitForDelivery: true,
	})
	return sentryHandler
}

func (s *APIGatewayService) Start(port string) error {
	godotenv.Load()

	log.Println("Starting API Gateway Service on Port:", port)

	s.router = mux.NewRouter()
	s.server = &http.Server{
		Addr:    ":" + port,
		Handler: s.router,
	}
	s.registerRoutes()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server ListenAndServe error: %v\n", err)
	}

	return nil
}

func (s *APIGatewayService) Stop() error {
	// Stop the API Gateway service
	log.Println("Stopping API Gateway")
	return nil
}
