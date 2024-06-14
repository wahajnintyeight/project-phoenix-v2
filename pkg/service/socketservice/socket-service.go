package service

import (
	// "context"

	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"project-phoenix/v2/internal/broker"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/helper"
	"project-phoenix/v2/pkg/service"
	"reflect"
	"sync"
	"time"

	"github.com/go-micro/plugins/v4/broker/rabbitmq"
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

const (
	MaxRetries  = 6
	RetryDelay  = 2 * time.Second
	serviceName = "location-service"
)

var rooms = make(map[string]*Room)
var upgrader = websocket.Upgrader{}

var socketOnce sync.Once

func (ss *SocketService) GetSubscribedTopics() []internal.SubscribedServices {
	serviceConfig, e := internal.ReturnServiceConfig(serviceName)
	if e != nil {
		log.Println("Unable to read service config", e)
		return nil
	}
	ss.subscribedServices = serviceConfig.(*internal.ServiceConfig).SubscribedServices
	return ss.subscribedServices
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

// attemptSubscribe tries to subscribe to a topic with retries until successful or max retries reached.
func (ss *SocketService) attemptSubscribe(queueName string, topic internal.SubscribedTopicsMap) error {
	log.Println("Max Retries:", MaxRetries)
	for i := 0; i <= MaxRetries; i++ {
		log.Println("Attempting to subscribe to", topic, " for Queue", queueName)
		if err := ss.subscribeToTopic(queueName, topic); err != nil {
			if err.Error() == "not connected" && i < MaxRetries {
				log.Printf("Broker not connected, retrying %d/%d for topic %s", i+1, MaxRetries, topic.TopicName)
				time.Sleep(RetryDelay)
				continue
			}
			return err
		}
		break
	}
	return nil
}

func (ss *SocketService) subscribeToTopic(queueName string, topic internal.SubscribedTopicsMap) error {
	handler, ok := reflect.TypeOf(ss).MethodByName(topic.TopicHandler)
	if !ok {
		return fmt.Errorf("Handler method %s not found for topic %s", topic.TopicHandler, topic.TopicName)
	}

	_, err := ss.brokerObj.Subscribe(topic.TopicName, func(p microBroker.Event) error {
		returnValues := handler.Func.Call([]reflect.Value{reflect.ValueOf(ss), reflect.ValueOf(p)})
		if err, ok := returnValues[0].Interface().(error); ok && err != nil {
			return err
		}
		return nil
	}, microBroker.Queue(queueName), rabbitmq.DurableQueue())

	if err != nil {
		log.Printf("Failed to subscribe to topic %s due to error: %v", topic.TopicName, err)
		return err
	}

	log.Printf("Successfully subscribed to topic %s | Handler: %s", topic.TopicName, topic.TopicHandler)
	return nil
}

func (ss *SocketService) ListenSubscribedTopics(broker microBroker.Event) error {
	log.Println("Broker Event: ", broker.Message().Header)

	return nil
}

func (ss *SocketService) HandleTripStart(p microBroker.Event) error {
	log.Println("Handle Trip Start Function | Data: ", p.Message().Header, " | Body: ", p.Message().Body)
	data := make(map[string]interface{})
	if err := json.Unmarshal(p.Message().Body, &data); err != nil {
		log.Println("Error occurred while unmarshalling the data", err)
	}
	log.Println("Data Received: ", data)
	ss.Broadcast(getSocketRoom(data["userId"].(string), data["tripId"].(string)), map[string]interface{}{"action": "trip-started", "data": "Trip Started"}, ss.socketObj)
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
				log.Println("Identify User Event")
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
			default:
				log.Println("No Action Found", action)
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
		if identifyUser.UserId == "" {
			//kick the user out
			log.Println("User not identified")
			defer conn.Close()
			return
		} else {
			log.Println("User ", identifyUser.UserId, " joined the room - ", getSocketRoom(identifyUser.UserId, identifyUser.TripId))
			ss.JoinRoom(getSocketRoom(identifyUser.UserId, identifyUser.TripId), conn)
		}
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

		//Publish the message back to location service to store it.
		broker.CreateBroker(enum.RABBITMQ).PublishMessage(dat, ss.serviceConfig.ServiceQueue, "process-location")
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

	http.Handle("/socket.io/", server)
	log.Fatal(http.ListenAndServe(":"+ss.serviceConfig.Port, nil))

}

func (ss *SocketService) InitServer() {
	// Initialize socket.io server
	http.HandleFunc("/socket.io", ss.HandleConnections)
	http.HandleFunc("/", ss.HandleConnections)

}

func (ss *SocketService) SubscribeTopics() {
	ss.InitServiceConfig()
	for _, service := range ss.serviceConfig.SubscribedServices {
		log.Println("Service", service)
		for _, topic := range service.SubscribedTopics {
			log.Println("Preparing to subscribe to service: ", service.Name, " | Topic: ", topic.TopicName, " | Queue: ", service.Queue, " | Handler: ", topic.TopicHandler, " | MaxRetries: ", MaxRetries)
			if err := ss.attemptSubscribe(service.Queue, topic); err != nil {
				log.Printf("Subscription failed for topic %s: %v", topic.TopicName, err)
			}
		}
	}

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
	return
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
			Addr:    "0.0.0.0:" + s.serviceConfig.Port,
			Handler: handlers.CORS(handlers.AllowedOrigins([]string{"*"}))(s.router), // Allow all origins
		}
		log.Println("Running on port: ", s.server.Addr, s.serviceConfig.Port)
		log.Fatal(http.ListenAndServe("0.0.0.0:"+s.serviceConfig.Port, nil))

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
