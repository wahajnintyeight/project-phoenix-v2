package service

import (
	// "context"

	"log"
	"net/http"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/helper"
	"project-phoenix/v2/pkg/service"
	"sync"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
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
	socketObj          *websocket.Conn
}

// Room struct to manage clients
type Room struct {
	clients map[*websocket.Conn]bool
	mu      sync.Mutex
}

var rooms = make(map[string]*Room)
var upgrader = websocket.Upgrader{}

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

func (ss *SocketService) HandleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal("Upgrade:", err)
	}
	defer conn.Close()

	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		roomID = "default"
	}

	ss.JoinRoom(roomID, conn)

	for {
		var msg map[string]string
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("Read error:", err)
			ss.RemoveClient(roomID, conn)
			break
		}

		if action, ok := msg["action"]; ok && action == "broadcast" {
			ss.Broadcast(roomID, msg)
		}
		if action, ok := msg["action"]; ok {
			switch action {
			case "identifyUser":
				handleIdentifyUser(roomID, conn, msg)
			case "broadcast":
				ss.Broadcast(roomID, msg)
			}
		}
	}
}

func handleIdentifyUser(roomID string, conn *websocket.Conn, msg map[string]string) {
	// log.Println("Identify User Event:", msg)
	dat, e := helper.StructToMap(msg)
	if e != nil {
		log.Println(e)
	} else {
		log.Println(dat["userId"])
	}
	// Handle the identifyUser event (e.g., log the user info or perform some action)
}

func (ss *SocketService) InitServer() {
	// Initialize socket.io server
	http.HandleFunc("/socket.io", ss.HandleConnections)
	http.HandleFunc("/", ss.HandleConnections)
	// server := socketio.NewServer(nil)
	// log.Println("Server Instance: ", server)
	// // ss.socketObj = server

	// ss.socketObj.OnConnect("/", func(s socketio.Conn) error {
	// 	s.SetContext("")
	// 	log.Println("Connected:", s.ID())
	// 	s.Emit("/connected", "Connected to the server")
	// 	return nil
	// })
	// ss.socketObj.OnDisconnect("/", func(s socketio.Conn, reason string) {
	// 	log.Println("Disconnected:", reason)
	// })
	// ss.socketObj.OnEvent("/", "notice", func(s socketio.Conn, msg string) {
	// 	log.Println("Notice event", msg)
	// })
	// server.OnEvent("notice", "notice", func(s socketio.Conn, data string) {
	// 	log.Println("notice event", data)
	// })
	// ss.socketObj.OnEvent("/", "identifyUser", func(s socketio.Conn, msg string) {
	// 	log.Println("Identify User Event", msg)
	// })

}

func (ss *SocketService) SubscribeTopics() {
	ss.InitServiceConfig()

}

func (ss *SocketService) JoinRoom(roomID string, conn *websocket.Conn) {
	room, exists := rooms[roomID]
	if !exists {
		room = &Room{clients: make(map[*websocket.Conn]bool)}
		rooms[roomID] = room
	}

	room.mu.Lock()
	room.clients[conn] = true
	room.mu.Unlock()
}

func (ss *SocketService) RemoveClient(roomID string, conn *websocket.Conn) {
	if room, exists := rooms[roomID]; exists {
		room.mu.Lock()
		delete(room.clients, conn)
		room.mu.Unlock()
	}
}

func (ss *SocketService) Broadcast(roomID string, msg map[string]string) {
	if room, exists := rooms[roomID]; exists {
		room.mu.Lock()
		defer room.mu.Unlock()
		for client := range room.clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Println("Write error:", err)
				client.Close()
				delete(room.clients, client)
			}
		}
	}
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
	// s.router.Handle("/socket.io/", http.StripPrefix("/socket.io", s.socketObj))
	// s.router.Handle("/", http.FileServer(http.Dir("./public")))

	s.server = &http.Server{
		Addr:    ":" + s.serviceConfig.Port,
		Handler: handlers.CORS(handlers.AllowedOrigins([]string{"*"}))(s.router), // Allow all origins
	}

	// go func() {
	// 	if err := s.socketObj.Serve(); err != nil {
	// 		log.Fatalf("SocketIO listen error: %s\n", err)
	// 	}
	// }()

	log.Fatal(http.ListenAndServe(":9000", nil))
	// if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
	// 	log.Fatalf("ListenAndServe(): %s\n", err)
	// }
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
