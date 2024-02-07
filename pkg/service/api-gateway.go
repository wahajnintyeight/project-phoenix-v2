package service

import (
	"fmt"
	"net/http"
	SessionController "project-phoenix/v2/internal/controllers"
	"sync"

	"github.com/gorilla/mux"
	"go-micro.dev/v4"
)

type APIGatewayService struct {
	service micro.Service
	router  *mux.Router
}

var apiGatewayServiceObj *APIGatewayService
var once sync.Once

func InitializeService(serviceName string) *APIGatewayService {

	once.Do(func() {
		service := micro.NewService(micro.Name(serviceName))
		router := mux.NewRouter()
		router.HandleFunc("/v2/api", PUTRoutes).Methods("PUT")
		apiGatewayServiceObj = &APIGatewayService{
			service: service,
		}
	})
	return apiGatewayServiceObj
}

func PUTRoutes(w http.ResponseWriter, r *http.Request) {
	//switch case for handling all the PUT routes
	urlPath := r.URL.Path

	switch urlPath {
	case "/createSession":
		fmt.Println("Create Session")
		sessionController := SessionController.SessionController{}
		sessionController.CreateSession(w, r)
		break
	default:
		http.NotFound(w, r)
	}
}

func (s *APIGatewayService) Start() error {
	fmt.Println("Starting API Gateway Service")
	if err := s.service.Run(); err != nil {
		return err
	}
	return nil
}

func (s *APIGatewayService) Stop() error {
	// Stop the API Gateway service
	// Implementation depends on how you manage service lifecycle
	return nil
}
