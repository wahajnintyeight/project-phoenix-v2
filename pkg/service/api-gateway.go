package service

import (
	// "context"
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"os"
	"os/signal"
	"project-phoenix/v2/internal/controllers/middleware"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/handler"
	"sync"

	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go-micro.dev/v4"
	// "log"
)

type APIGatewayService struct {
	service       micro.Service
	router        *mux.Router
	server        *http.Server
	serviceConfig internal.ServiceConfig
}

var once sync.Once

func (api *APIGatewayService) InitializeService(serviceObj micro.Service, serviceName string) ServiceInterface {

	once.Do(func() {
		service := serviceObj
		api.service = service
		api.router = mux.NewRouter()
		godotenv.Load()
		servicePath := os.Getenv("API_GATEWAY_SERVICE_CONFIG_PATH")
		serviceConfig, _ := internal.ReturnServiceConfig(servicePath)
		api.serviceConfig = serviceConfig.(internal.ServiceConfig)
	})

	return api
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
	serviceConfig, serviceConfigErr := internal.ReturnServiceConfig(os.Getenv("API_GATEWAY_SERVICE_CONFIG_PATH"))
	fmt.Println("Starting API Gateway Service on", serviceConfig.(internal.ServiceConfig).Port)
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

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe error: %v\n", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop // Block until a signal is received

	log.Println("Shutting down the server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v\n", err)
	}

	log.Println("Server gracefully stopped")
	return nil
}

func (s *APIGatewayService) Stop() error {
	// Stop the API Gateway service
	// Implementation depends on how you manage service lifecycle
	return nil
}
