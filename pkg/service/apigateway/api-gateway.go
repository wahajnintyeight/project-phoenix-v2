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

	"github.com/gorilla/handlers"
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
		s.serviceConfig.EndpointPrefix + "/capture-screen",
		s.serviceConfig.EndpointPrefix + "/scan-devices",
		s.serviceConfig.EndpointPrefix + "/ping",
		s.serviceConfig.EndpointPrefix + "/devices",
		s.serviceConfig.EndpointPrefix + "/device", //deleting a device
		s.serviceConfig.EndpointPrefix + "/room/create",
		s.serviceConfig.EndpointPrefix + "/room/join",
		s.serviceConfig.EndpointPrefix + "/room/update",
		s.serviceConfig.EndpointPrefix + "/room/messages",
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

	allowedOrigins := []string{"http://localhost:5173","http://localhost:5173/", "electron://altair"}

    // Set up CORS middleware
    corsMiddleware := handlers.CORS(
        handlers.AllowedOrigins(allowedOrigins),
        handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE"}), // Adjust methods as needed
        handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}), // Adjust headers as needed
		handlers.AllowCredentials(),
    )

    // Apply CORS middleware to the router
    s.router.Use(corsMiddleware)

	s.router.Use(s.ConfigureSentry().Handle)
	s.router.Use(customMiddlewareWrapper)
	s.router.Use(customMiddlewareWrapperWithSession)
	s.router.PathPrefix(s.serviceConfig.EndpointPrefix).Handler(apiRequestHandler)
}


func (s *APIGatewayService) ConfigureSentry() *sentryhttp.Handler {

	sentryDsn := os.Getenv("SENTRY_DSN")
	if err := sentry.Init(sentry.ClientOptions{
		Dsn: sentryDsn,
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
