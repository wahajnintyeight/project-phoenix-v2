package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

type SSERequestHandler struct {
	clients map[chan map[string]interface{}]bool
	// Add client routing support
	clientRoutes    map[chan map[string]interface{}]map[string]bool // client -> route keys
	addClient       chan chan map[string]interface{}
	removeClient    chan chan map[string]interface{}
	broadcast       chan map[string]interface{}
	mutex           sync.Mutex
}

var sseRequestHandlerObj SSERequestHandler

// filterLogMessage removes base64 content from log messages to avoid cluttering logs
func filterLogMessage(message map[string]interface{}) map[string]interface{} {
	const redacted = "[base64 data omitted]"

	var cleanse func(v interface{}) interface{}
	cleanse = func(v interface{}) interface{} {
		switch t := v.(type) {
		case map[string]interface{}:
			out := make(map[string]interface{}, len(t))
			for k, val := range t {
				switch k {
				case "fileContent", "fileData", "chunkData":
					out[k] = redacted
				case "file":
					// handle a nested file map or array of files
					switch fv := val.(type) {
					case map[string]interface{}:
						out[k] = cleanse(fv).(map[string]interface{})
						// ensure base64 fields are redacted inside "file"
						if m, ok := out[k].(map[string]interface{}); ok {
							if _, ok := m["fileContent"]; ok {
								m["fileContent"] = redacted
							}
							if _, ok := m["fileData"]; ok {
								m["fileData"] = redacted
							}
						}
					case []interface{}:
						arr := make([]interface{}, len(fv))
						for i, iv := range fv {
							c := cleanse(iv)
							// ensure redaction inside each file map
							if m, ok := c.(map[string]interface{}); ok {
								if _, ok := m["fileContent"]; ok {
									m["fileContent"] = redacted
								}
								if _, ok := m["fileData"]; ok {
									m["fileData"] = redacted
								}
							}
							arr[i] = c
						}
						out[k] = arr
					default:
						out[k] = val
					}
				default:
					out[k] = cleanse(val)
				}
			}
			return out
		case []interface{}:
			out := make([]interface{}, len(t))
			for i, iv := range t {
				out[i] = cleanse(iv)
			}
			return out
		default:
			return v
		}
	}

	return cleanse(message).(map[string]interface{})
}

func NewSSERequestHandler() *SSERequestHandler {
	handler := &SSERequestHandler{
		clients:      make(map[chan map[string]interface{}]bool),
		clientRoutes: make(map[chan map[string]interface{}]map[string]bool),
		addClient:    make(chan chan map[string]interface{}),
		removeClient: make(chan chan map[string]interface{}),
		broadcast:    make(chan map[string]interface{}),
	}

	go handler.Run()

	return handler
}

func (handler *SSERequestHandler) Run() {
	for {
		select {
		case client := <-handler.addClient:
			handler.mutex.Lock()
			handler.clients[client] = true
			handler.mutex.Unlock()
			log.Println("New client connected")
		case client := <-handler.removeClient:
			log.Println("Remove client called")
			handler.mutex.Lock()
			if _, ok := handler.clients[client]; ok {
				delete(handler.clients, client)
				delete(handler.clientRoutes, client)
				close(client)
				log.Println("Client disconnected")
			}
			handler.mutex.Unlock()
		case msg := <-handler.broadcast:
			log.Printf("Broadcast message received: type=%s, progress=%v", msg["type"], msg["progress"])
			handler.mutex.Lock()

			// Check if message has routing info
			routeKey, hasRoute := msg["routeKey"].(string)
			log.Printf("HANDLER CLIENTS", handler.clients)
			for client := range handler.clients {
				shouldSend := true
				log.Printf("HAS ROUTE", hasRoute)
				// If message has routing, check if client subscribed to this route
				if hasRoute {
					if routes, exists := handler.clientRoutes[client]; exists {
						shouldSend = routes[routeKey]
					} else {
						shouldSend = false // Client not subscribed to any routes
					}
				}
				// If no routing info, broadcast to all (backward compatibility)
				log.Printf("SHOULD SEND", shouldSend)
				// if shouldSend {
				// 	select {
				// 	case client <- msg:
				// 		log.Println("[RUN]Message sent to client:", filterLogMessage(msg))
				// 	default:
				// 		// close(client)
				// 		// delete(handler.clients, client)
				// 		// delete(handler.clientRoutes, client)
				// 		log.Printf("Client is busy, dropping message for client %v", client)
				// 	}
				// }
				client <- msg
			}
			handler.mutex.Unlock()
		default:
			// log.Println("Default case",handler.clients)
		}
	}
}

func (handler *SSERequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	log.Println("Serve HTTP | Method: ", r.Method)

	// Create a new client channel
	clientChan := make(chan map[string]interface{})

	// Add this client to our map
	handler.addClient <- clientChan

	// Remove client when connection is closed
	defer func() {
		handler.removeClient <- clientChan
	}()

	for {
		select {
		case msg := <-clientChan:

			// Serialize the message to JSON
			jsonData, err := json.Marshal(msg)
			if err != nil {
				log.Printf("Error marshalling message to JSON: %v", err)
				return
			}
			// Send the JSON data as a string
			_, err = fmt.Fprintf(w, "data: %s\n\n", jsonData)
			if err != nil {
				log.Printf("Error sending message to client: %v", err)
				return
			}
			log.Println("Message sent to client:", w)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// Public method to add client with route subscription
// Note: No message replay - SSE is real-time. Clients connecting late should handle their own state recovery.
func (handler *SSERequestHandler) AddClientWithRoute(routeKey string) chan map[string]interface{} {
	clientChan := make(chan map[string]interface{}, 1000)
	handler.addClient <- clientChan
	handler.SubscribeClientToRoute(clientChan, routeKey)
	log.Printf("[HANDLER] Client connected to route: %s", routeKey)
	return clientChan
}

// Public method to remove client
func (handler *SSERequestHandler) RemoveClient(clientChan chan map[string]interface{}) {
	handler.removeClient <- clientChan
}

func (handler *SSERequestHandler) Broadcast(message map[string]interface{}) {
	handler.broadcast <- message
	log.Println("Broadcast message:", message)
}

// Subscribe client to specific routes (for targeted messaging)
func (handler *SSERequestHandler) SubscribeClientToRoute(client chan map[string]interface{}, routeKey string) {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()

	if handler.clientRoutes[client] == nil {
		handler.clientRoutes[client] = make(map[string]bool)
	}
	handler.clientRoutes[client][routeKey] = true
	log.Printf("Client subscribed to route: %s", routeKey)
}

// Broadcast to specific route only
// Note: No buffering - messages are real-time only. Reduces memory usage and prevents
// issues with large file chunks being replayed incorrectly.
func (handler *SSERequestHandler) BroadcastToRoute(routeKey string, message map[string]interface{}) {
	message["routeKey"] = routeKey
	handler.broadcast <- message
}
