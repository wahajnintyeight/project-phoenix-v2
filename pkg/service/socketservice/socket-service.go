package service

import (
	// "context"

	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"project-phoenix/v2/internal/broker"
	"project-phoenix/v2/internal/controllers"
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
var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

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

func (ss *SocketService) HandleTripEnded(p microBroker.Event) error {
	log.Println("Handle Trip Start Function | Data: ", p.Message().Header, " | Body: ", p.Message().Body)
	data := make(map[string]interface{})
	if err := json.Unmarshal(p.Message().Body, &data); err != nil {
		log.Println("Error occurred while unmarshalling the data", err)
	}
	log.Println("Data Received: ", data)
	ss.Broadcast(getSocketRoom(data["userId"].(string), data["tripId"].(string)), map[string]interface{}{"action": "trip-ended", "data": "Trip Stopped"}, ss.socketObj)
	return nil
}

func (ss *SocketService) HandleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal("Upgrade:", err)
		return
	}

	log.Println(conn.LocalAddr(), conn.RemoteAddr(), conn.LocalAddr().String())
	ss.socketObj = conn

	defer func() {
		// Clean up connection
		for roomID, room := range rooms {
			room.mu.Lock()
			if _, exists := room.clients[conn]; exists {
				ss.RemoveClient(roomID, conn)
				log.Printf("Cleaned up client from room %s on connection close", roomID)
			}
			room.mu.Unlock()
		}
		conn.Close()
	}()

	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("Connection closed unexpectedly: %v", err)
				// Handle code 1006 (abnormal closure) specifically
				if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
					log.Printf("Abnormal closure (1006) detected. Client may have disconnected without proper close frame.")
				}
				// Don't immediately remove the client - let the defer function handle cleanup
			} else {
				log.Printf("Read error: %v", err)
			}
			break
		}

		// log.Println("JSON READ:", msg)
		log.Println("[DEBUG] Message Received from client: ", msg)
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
				ss.handleDisconnect(conn, msg)
			case "notice":
				log.Println("Notice Event", msg)
			case "locationUpdate":
				log.Println("Location Update Event Received")
				ss.handleLocationUpdate(conn, msg)
			case "joinClipRoom":
				log.Println("Join Clipboard Room Event Received")
				ss.handleClipRoomJoined(conn, msg)
			case "sendRoomMessage":
				log.Println("Send Room Message Event Fetched")
				ss.handleSendRoomMessage(conn, msg)
			default:
				log.Println("No Action Found", action)
			}
		}
	}
}

func (ss *SocketService) handleDisconnect(conn *websocket.Conn, msg map[string]interface{}) {
	dat, err := helper.StructToMap(msg)
	if err != nil {
		log.Println("Error parsing disconnect message:", err)
		return
	}

	if data, ok := dat["data"].(map[string]interface{}); ok {
		if roomID, exists := data["roomId"].(string); exists {
			log.Printf("Client disconnecting from room %s", roomID)
			
			// Check if this is a clipboard room - we need to translate the roomId
			if code, hasCode := data["code"].(string); hasCode {
				// Check if it's anonymous or not
				isAnonymous, _ := data["isAnonymous"].(bool)
				userId, _ := data["userId"].(string)
				
				if isAnonymous {
					roomID = getAnonymousClipBoardRoom(code)
				} else {
					roomID = getClipBoardRoom(code, userId)
				}
				log.Printf("Translated roomId to %s", roomID)
			}

			// Send close message first
			deadline := time.Now().Add(time.Second)
			err := conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				deadline,
			)
			if err != nil {
				log.Printf("Error sending close message: %v", err)
			}

			// Remove from room
			ss.RemoveClient(roomID, conn)

			// Set read deadline for clean shutdown
			conn.SetReadDeadline(time.Now().Add(time.Second))

			// Wait for close message or timeout
			for {
				_, _, err := conn.NextReader()
				if err != nil {
					break
				}
			}

			// Finally close the connection
			conn.Close()

			// Cleanup empty room
			if room, exists := rooms[roomID]; exists {
				room.mu.Lock()
				if len(room.clients) == 0 {
					delete(rooms, roomID)
					log.Printf("Room %s removed as it has no clients", roomID)
				}
				room.mu.Unlock()
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

func (ss *SocketService) handleSendRoomMessage(conn *websocket.Conn, msg map[string]interface{}) {
	dat, e := helper.StructToMap(msg)
	clipBoardRoom := &model.ClipBoardSendRoomMessage{}
	log.Println("Data:", dat)

	if e != nil {
		log.Println(e)
		return
	} else {
		er := helper.InterfaceToStruct(dat["data"], &clipBoardRoom)
		if er != nil {
			log.Println(er)
		}

		// log.Println("", clipBoardRoom.Code, " joined the room - ", getClipBoardRoom(clipBoardRoom.Code))
		if clipBoardRoom.IsAnonymous == false {
			log.Println("Broadcasting to user clipboard room", clipBoardRoom.Sender)
			ss.Broadcast(getClipBoardRoom(clipBoardRoom.Code, clipBoardRoom.Sender), msg, conn)
		} else {
			log.Println("Broadcasting to anonymous clipboard room")
			ss.Broadcast(getAnonymousClipBoardRoom(clipBoardRoom.Code), msg, conn)
		}
		controller := controllers.GetControllerInstance(enum.ClipboardRoomController, enum.MONGODB)
		clipboardRoomController := controller.(*controllers.ClipboardRoomController)
		clipboardRoomController.ProcessRoomMessage(clipBoardRoom.Code, msg)
	}
}

func (ss *SocketService) handleClipRoomJoined(conn *websocket.Conn, msg map[string]interface{}) {
	dat, e := helper.StructToMap(msg)
	clipBoardRoom := &model.ClipBoardRoomJoined{}
	log.Println("Data:", dat)

	if e != nil {
		log.Println(e)
		return
	} else {
		er := helper.InterfaceToStruct(dat["data"], &clipBoardRoom)
		if er != nil {
			log.Println(er)
		}
		if clipBoardRoom.Code == "" {
			//kick the user out
			log.Println("Code not found")
			defer conn.Close()
			return
		} else {
			if clipBoardRoom.IsAnonymous {
				log.Println("Anonymous User has joined the room - ", getAnonymousClipBoardRoom(clipBoardRoom.Code))
				getAnonymousClipBoardRoom(clipBoardRoom.Code)
				ss.JoinRoom(getAnonymousClipBoardRoom(clipBoardRoom.Code), conn)
			} else {
				log.Println("User ", clipBoardRoom.UserId, " joined the room - ", getClipBoardRoom(clipBoardRoom.Code, clipBoardRoom.UserId))
				getClipBoardRoom(clipBoardRoom.Code, clipBoardRoom.UserId)
				ss.JoinRoom(getClipBoardRoom(clipBoardRoom.Code, clipBoardRoom.UserId), conn)
			}
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

func getAnonymousClipBoardRoom(code string) string {
	return "room-" + code
}

func getClipBoardRoom(code string, userId string) string {
	return "room-" + code + "-" + userId
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
	log.Printf("Attempting to join room %s for connection %v", roomID, conn.RemoteAddr())

	room, exists := rooms[roomID]
	if !exists {
		room = &Room{clients: make(map[*websocket.Conn]bool)}
		rooms[roomID] = room
		log.Printf("Created new room %s", roomID)
	}

	// Use defer to ensure the mutex is always unlocked
	room.mu.Lock()
	defer room.mu.Unlock()
	
	// Check if connection is already in room
	if _, already := room.clients[conn]; already {
		log.Printf("Connection %v is already in room %s", conn.RemoteAddr(), roomID)
	} else {
		room.clients[conn] = true
		clientCount := len(room.clients)
		log.Printf("Added to room %s. Current clients: %d", roomID, clientCount)
	}

	log.Printf("Successfully joined room %s.", roomID)
}

func (ss *SocketService) RemoveClient(roomID string, conn *websocket.Conn) {
	if room, exists := rooms[roomID]; exists {
		room.mu.Lock()
		defer room.mu.Unlock()
		
		if _, found := room.clients[conn]; found {
			delete(room.clients, conn)
			clientCount := len(room.clients)
			log.Printf("Client removed from room %s. Remaining clients: %d", roomID, clientCount)
		} else {
			log.Printf("Client not found in room %s", roomID)
		}
		
		// Cleanup empty room
		if len(room.clients) == 0 {
			delete(rooms, roomID)
			log.Printf("Room %s removed as it has no clients", roomID)
		}
	} else {
		log.Printf("Room %s not found for client removal", roomID)
	}
}

func (ss *SocketService) Broadcast(roomID string, msg map[string]interface{}, sender *websocket.Conn) {
	if room, exists := rooms[roomID]; exists {
		room.mu.Lock()
		defer room.mu.Unlock()
		broadcastCount := 0
		for client := range room.clients {
			if client != sender {
				log.Println("Broadcasting to User", client.LocalAddr(), client.RemoteAddr())
				err := client.WriteJSON(msg)
				if err != nil {
					log.Println("Write error:", err)
					// Don't remove client immediately on write error - it might be temporary
					// Instead, check if it's a fatal error
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
						log.Printf("Client connection appears to be closed, removing from room %s", roomID)
						delete(room.clients, client)
						client.Close()
					} else {
						log.Printf("Non-fatal write error to client: %v", err)
					}
				} else {
					broadcastCount++
				}
			}
		}
		log.Printf("Broadcasted message to %d clients in room %s", broadcastCount, roomID)
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
		s.server = &http.Server{
			Addr:    "0.0.0.0:" + port,
			Handler: handlers.CORS(handlers.AllowedOrigins([]string{"*"}))(s.router), // Allow all origins
		}
		s.InitServer()
		log.Println("Running on port: ", s.server.Addr, port)
		log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))

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
