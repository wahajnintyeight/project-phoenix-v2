package service

import (
	// "context"
	"context"
	"fmt"
	"log"
	"net/http"

	"os"
	"os/signal"
	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/response"
	internal "project-phoenix/v2/internal/service-configs"
	"sync"

	"syscall"
	"time"

	"github.com/gorilla/mux"
	"go-micro.dev/v4"
	// "log"
)

type APIGatewayService struct {
	service       micro.Service
	router        *mux.Router
	server        *http.Server
	serviceConfig internal.ServiceConfig
}

// var apiGatewayServiceObj *APIGatewayService
var once sync.Once

func (api *APIGatewayService) InitializeService(serviceObj micro.Service, serviceName string) ServiceInterface {

	once.Do(func() {
		service := serviceObj
		api.service = service
		api.router = mux.NewRouter()
		serviceConfig, _ := internal.ReturnServiceConfig("internal/service-configs/api-gateway/service-config.json")
		api.serviceConfig = serviceConfig.(internal.ServiceConfig)
	})
	// api.service.Init()

	return api
}

func NewAPIGatewayService(serviceObj micro.Service, serviceName string) ServiceInterface {
	apiGatewayService := &APIGatewayService{}
	return apiGatewayService.InitializeService(serviceObj, serviceName)
}

func (api *APIGatewayService) SessionRoutes(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	action := vars["action"]
	log.Println("API Called: ", r.URL.Path)
	switch action {
	case "create":
		controller := controllers.GetControllerInstance(enum.SessionController, enum.MONGODB, "sessions")
		sessionController := controller.(*controllers.SessionController)
		res, ok := sessionController.CreateSession(w, r)
		if ok != nil {
			response.SendResponse(w, int(enum.SESSION_NOT_CREATED), res)
		} else {
			fmt.Println("Session created successfully", res)
			response.SendResponse(w, int(enum.SESSION_CREATED), res)
			return
		}
		break
	case "delete":
		log.Println("Delete API Called")

		break
	default:
		http.NotFound(w, r)
	}
}

func GETRoutes(w http.ResponseWriter, r *http.Request) {
	//switch case for handling all the GET routes
	urlPath := r.URL.Path 
	log.Print("GET Routes: ", urlPath)
	switch urlPath {
	case urlPath + "/":
		log.Print("Welcome to API Gateway")
		response.SendResponse(w, 1000, "Welcome to API Gateway")
		break
	default:
		http.NotFound(w, r)
	}
}

func (s *APIGatewayService) registerRoutes() {
	s.router.HandleFunc(s.serviceConfig.EndpointPrefix+"/session/{action}", s.SessionRoutes).Methods("PUT")
	s.router.HandleFunc(s.serviceConfig.EndpointPrefix + "/", GETRoutes).Methods("GET")
}

func (s *APIGatewayService) Start() error {
	serviceConfig, serviceConfigErr := internal.ReturnServiceConfig("internal/service-configs/api-gateway/service-config.json")
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
