package main

import (
	"fmt"
	"log"

	socketio "github.com/zhouhui8915/go-socket.io-client"
)

func main() {
	opts := &socketio.Options{
		Transport: "websocket",
		Query:     make(map[string]string),
	}
	client, err := socketio.NewClient("http://127.0.0.1:9000", opts)
	log.Println("Client Instance: ", client)
	if err != nil {
		log.Fatalf("Error creating client: %v", err)
	}
	go func() {
		client.Emit("identifyUser", map[string]interface{}{"userId": "3275832u5893u5834", "tripId": "421794729347324"})
	}()
	client.On("connection", func() {
		fmt.Println("Connected to the server")
		client.Emit("notice", "Hello from Go client!")
	})

	client.On("connected", func() {
		fmt.Println("Connected to the server")
		client.Emit("notice", "Hello from Go client!")
	})

	client.On("reply", func(msg string) {
		fmt.Printf("Server reply: %s\n", msg)
	})

	client.On("disconnection", func() {
		fmt.Println("Disconnected from server")
	})

	client.On("error", func(err string) {
		fmt.Println("Error:", err)
	})

	// Keep the client running
	select {}
}
