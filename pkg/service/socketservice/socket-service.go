package service

import (
	// "context"

	"log"
	"net/http"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/service"
	"sync"

	socketio "github.com/googollee/go-socket.io"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go-micro.dev/v4"
	microBroker "go-micro.dev/v4/broker"
)

type SocketService struct {
	service            micro.Service
	router             *mux.Router
	server             *http.Server
	serviceConfig      internal.ServiceConfig
	subscribedServices []internal.SubscribedServices
	brokerObj          microBroker.Broker
	socketObj          *socketio.Server
}

var socketOnce sync.Once

func (ss *SocketService) GetSubscribedTopics() []internal.SubscribedServices {
	return nil
}

func (ss *SocketService) InitializeService(serviceObj micro.Service, serviceName string) service.ServiceInterface {
	socketOnce.Do(func() {
		service := serviceObj
		ss.service = service
		ss.router = mux.NewRouter()
		ss.brokerObj = service.Options().Broker
		godotenv.Load()
		servicePath := "socket"
		serviceConfig, _ := internal.ReturnServiceConfig(servicePath)
		ss.serviceConfig = serviceConfig.(internal.ServiceConfig)
		ss.InitServiceConfig()
		log.Println("Socket Service Config", ss.serviceConfig)
		log.Println("Socket Service Broker Instance: ", ss.brokerObj)
	})
	return ss
}

func (ss *SocketService) ListenSubscribedTopics(broker microBroker.Event) error {

	return nil
}

func (ss *SocketService) InitServer() {
	// Initialize socket.io server
	server := socketio.NewServer(nil)
	log.Println("Server Instance: ", server)
	ss.socketObj = server

	ss.socketObj.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		log.Println("Connected:", s.ID())
		s.Emit("/connected", "Connected to the server")
		return nil
	})
	ss.socketObj.OnDisconnect("/", func(s socketio.Conn, reason string) {
		log.Println("Disconnected:", reason)
	})
	ss.socketObj.OnEvent("/", "notice", func(s socketio.Conn, msg string) {
		log.Println("Notice event", msg)
	})

	ss.socketObj.OnEvent("/", "identifyUser", func(s socketio.Conn, msg string) {
		log.Println("Identify User Event", msg)
	})

}

func (ss *SocketService) SubscribeTopics() {
	ss.InitServiceConfig()

}

func (ls *SocketService) InitServiceConfig() {
	serviceConfig, er := internal.ReturnServiceConfig("socket")
	if er != nil {
		log.Println("Unable to read service config", er)
		return
	}
	ls.serviceConfig = serviceConfig.(internal.ServiceConfig)
}

func (s *SocketService) Start(port string) error {
	godotenv.Load()
	log.Println("Starting Socket Service on Port:", port)

	s.InitServer()

	// Setup router with socket.io and static file handlers
	s.router.Handle("/socket.io/", http.StripPrefix("/socket.io", s.socketObj))
	s.router.Handle("/", http.FileServer(http.Dir("./public")))

	s.server = &http.Server{
		Addr:    ":" + s.serviceConfig.Port,
		Handler: handlers.CORS(handlers.AllowedOrigins([]string{"*"}))(s.router), // Allow all origins
	}

	go func() {
		if err := s.socketObj.Serve(); err != nil {
			log.Fatalf("SocketIO listen error: %s\n", err)
		}
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe(): %s\n", err)
	}
	return nil
}

func NewSocketService(serviceObj micro.Service, serviceName string) service.ServiceInterface {
	socketService := &SocketService{}
	return socketService.InitializeService(serviceObj, serviceName)
}

func (s *SocketService) Stop() error {
	log.Println("Stopping Socket Service")
	return nil
}
