package service

import (
	// "context"

	"log"
	"net/http"
	"strings"
	"sync"

	"project-phoenix/v2/internal/controllers/middleware"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/handler"

	// "sync"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go-micro.dev/v4"
	microBroker "go-micro.dev/v4/broker"
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

func (api *APIGatewayService) InitializeService(serviceObj micro.Service, serviceName string) ServiceInterface {

	once.Do(func() {
		service := serviceObj
		api.service = service
		api.router = mux.NewRouter()
		api.brokerObj = service.Options().Broker
		godotenv.Load()
		servicePath := "api-gateway"
		serviceConfig, _ := internal.ReturnServiceConfig(servicePath)
		api.serviceConfig = serviceConfig.(internal.ServiceConfig)
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

func NewAPIGatewayService(serviceObj micro.Service, serviceName string) ServiceInterface {
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

	s.router.Use(customMiddlewareWrapper)
	s.router.Use(customMiddlewareWrapperWithSession)
	s.router.PathPrefix(s.serviceConfig.EndpointPrefix).Handler(apiRequestHandler)
}

func (s *APIGatewayService) Start() error {
	godotenv.Load()
	serviceConfig, serviceConfigErr := internal.ReturnServiceConfig("api-gateway")
	log.Println("Starting API Gateway Service on Port:", s.service.Server().Options().Address)
	var serverPort string
	if serviceConfigErr != nil {
		return serviceConfigErr
	} else {
		serverPort = serviceConfig.(internal.ServiceConfig).Port
	}
	s.router = mux.NewRouter()
	s.server = &http.Server{
		Addr:    ":" + serverPort,
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
