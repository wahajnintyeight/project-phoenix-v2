package service

import (
	// "context"

	"log"
	"net/http"
	"project-phoenix/v2/internal/model"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/helper"
	"project-phoenix/v2/pkg/service"
	"sync"

	socketio "github.com/googollee/go-socket.io"
	"github.com/googollee/go-socket.io/engineio"
	"github.com/googollee/go-socket.io/engineio/transport"
	"github.com/googollee/go-socket.io/engineio/transport/polling"
	ws "github.com/googollee/go-socket.io/engineio/transport/websocket"
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
	} else {
		log.Println(conn.LocalAddr(), conn.RemoteAddr(), conn.LocalAddr().String())
		ss.socketObj = conn
	}
	defer conn.Close()

	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("Read error:", err)
			// ss.RemoveClient(roomID, conn)
			break
		}
		log.Println("JSON READ:", msg)
		if action, ok := msg["action"]; ok {
			switch action {
			case "identifyUser":
				ss.handleIdentifyUser(conn, msg)
			case "connect":
				log.Println("Connected to the server | connect")
			case "connected":
				log.Println("Connected to the server | connected")
			case "disconnect":
				log.Println("Disconnected from the server", msg)
			case "notice":
				log.Println("Notice Event", msg)
			case "locationUpdate":
				log.Println("Location Update Event Received")
				ss.handleLocationUpdate(conn, msg)
			}
		}
	}
}

func (ss *SocketService) handleIdentifyUser(conn *websocket.Conn, msg map[string]interface{}) {
	dat, e := helper.StructToMap(msg)
	identifyUser := &model.IdentifyUser{}
	log.Println("Data:", dat)

	if e != nil {
		log.Println(e)
		return
	} else {
		er := helper.InterfaceToStruct(dat["data"], &identifyUser)
		if er != nil {
			log.Println(er)
		}
		log.Println("User ", identifyUser.UserId, " joined the room - ", getSocketRoom(identifyUser.UserId, identifyUser.TripId))
		ss.JoinRoom(getSocketRoom(identifyUser.UserId, identifyUser.TripId), conn)
		// ss.Broadcast(getSocketRoom(dat), msg)
	}
}

func (ss *SocketService) handleLocationUpdate(conn *websocket.Conn, msg map[string]interface{}) {
	dat, e := helper.StructToMap(msg)
	locationData := &model.LocationData{}
	if e != nil {
		log.Println(e)
	} else {
		er := helper.InterfaceToStruct(dat["data"], &locationData)
		if er != nil {
			log.Println(er)
		}
		log.Println("Location Data Received:", locationData)
		ss.Broadcast(getSocketRoom(locationData.UserId, locationData.TripId), msg, conn)
	}
}

func getSocketRoom(userId string, tripId string) string {
	roomName := "user-" + userId + "_" + tripId
	return roomName
}

var allowOriginFunc = func(r *http.Request) bool {
	return true
}

func (ss *SocketService) InitServerIO() {
	server := socketio.NewServer(&engineio.Options{
		Transports: []transport.Transport{
			&polling.Transport{
				CheckOrigin: allowOriginFunc,
			},
			&ws.Transport{
				CheckOrigin: allowOriginFunc,
			},
		},
	})
	log.Println("SOCKETIO | Server Instance: ", server)

	server.OnConnect("/", func(s socketio.Conn) error {
		// s.SetContext("")
		log.Println("Connected:", s.ID())
		s.Emit("/connected", map[string]interface{}{"message": "Connected to the server"})

		server.OnEvent("/", "disconnected", func(s socketio.Conn, msg string) {
			log.Println("Disconnected Event", msg)
		})
		log.Println("Total connections: ", server.Count())

		return nil
	})
	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		log.Println("Disconnected:", reason)
	})

	server.OnEvent("/", "identifyUser", func(s socketio.Conn, msg string) string {
		log.Println("Identify User Event", msg)
		return msg
	})

	server.OnEvent("/", "identifyUser", func(s socketio.Conn, msg string) {
		s.Emit("identifyUser", msg)
	})

	server.OnEvent("/", "disconnected", func(s socketio.Conn, msg string) {
		log.Println("Disconnected Event", msg)
	})

	// c := cors.New(cors.Options{
	// 	AllowedOrigins:   []string{"*"},
	// 	AllowCredentials: true,
	// })

	go func() {
		if err := server.Serve(); err != nil {
			log.Fatalf("Socket listen error: %s\n", err)
		}
	}()
	defer server.Close()

	// handler := c.Handler(server)
	http.Handle("/socket.io/", server)
	// http.Handle("/socket.io/", handler)
	// log.Fatal(http.ListenAndServe(":"+ss.serviceConfig.Port, handler))
	log.Fatal(http.ListenAndServe(":"+ss.serviceConfig.Port, nil))

}

func (ss *SocketService) InitServer() {
	// Initialize socket.io server
	http.HandleFunc("/socket.io", ss.HandleConnections)
	http.HandleFunc("/", ss.HandleConnections)

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
	log.Println("Room", rooms)
}

func (ss *SocketService) RemoveClient(roomID string, conn *websocket.Conn) {
	if room, exists := rooms[roomID]; exists {
		room.mu.Lock()
		delete(room.clients, conn)
		room.mu.Unlock()
	}
}

func (ss *SocketService) Broadcast(roomID string, msg map[string]interface{}, sender *websocket.Conn) {
	if room, exists := rooms[roomID]; exists {
		room.mu.Lock()
		defer room.mu.Unlock()
		for client := range room.clients {
			if client != sender {
				log.Println("Broadcasting to User", client.LocalAddr(), client.RemoteAddr())
				err := client.WriteJSON(msg)
				if err != nil {
					log.Println("Write error:", err)
					client.Close()
					delete(room.clients, client)
				}
			}
		}
	}
}

func (ss *SocketService) InitServiceConfig() {
	serviceConfig, er := internal.ReturnServiceConfig("socket")
	if er != nil {
		log.Println("Unable to read service config", er)
		return
	}
	ss.serviceConfig = serviceConfig.(internal.ServiceConfig)
}

func (s *SocketService) Start(port string) error {
	godotenv.Load()
	log.Println("Starting Socket Service on Port:", port)
	isIO := false
	if isIO {
		s.InitServerIO()
	} else {
		s.InitServer()
		s.server = &http.Server{
			Addr:    ":" + s.serviceConfig.Port,
			Handler: handlers.CORS(handlers.AllowedOrigins([]string{"*"}))(s.router), // Allow all origins
		}
		log.Println("Running on port: ", s.server.Addr, s.serviceConfig.Port)
		log.Fatal(http.ListenAndServe(":"+s.serviceConfig.Port, nil))

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
