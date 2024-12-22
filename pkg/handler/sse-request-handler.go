package handler

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)

type SSERequestHandler struct {
	clients      map[chan string]bool
	addClient    chan chan string
	removeClient chan chan string
	broadcast    chan string
	mutex        sync.Mutex
}

var sseRequestHandlerObj SSERequestHandler

func NewSSERequestHandler() *SSERequestHandler {
	handler := &SSERequestHandler{
		clients:      make(map[chan string]bool),
		addClient:    make(chan chan string),
		removeClient: make(chan chan string),
		broadcast:    make(chan string),
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
			handler.mutex.Lock()
			if _, ok := handler.clients[client]; ok {
				delete(handler.clients, client)
				close(client)
				log.Println("Client disconnected")
			}
			handler.mutex.Unlock()
		case msg := <-handler.broadcast:
			handler.mutex.Lock()
			for client := range handler.clients {
				select {
				case client <- msg:
				default:
					close(client)
					delete(handler.clients, client)
					log.Println("Client disconnected due to failure")
				}
			}
			handler.mutex.Unlock()
		}
	}
}


func (handler *SSERequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a new client channel
	clientChan := make(chan string)

	// Add this client to our map
	handler.addClient <- clientChan

	// Remove client when connection is closed
	defer func() {
		handler.removeClient <- clientChan
	}()

	 
	for {
		select {
		case msg := <-clientChan:
		 
			_, err := fmt.Fprintf(w, "data: %s\n\n", msg)
			if err != nil {
				log.Printf("Error sending message to client: %v", err)
				return
			}
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (handler *SSERequestHandler) Broadcast(message string) {
	handler.broadcast <- message
	log.Println("Broadcast message:", message)
}