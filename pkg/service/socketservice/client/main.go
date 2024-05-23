package main

import (
	"log"
	"net/url"

	"github.com/gorilla/websocket"
)

func main() {
	u := url.URL{Scheme: "ws", Host: "localhost:9000", Path: "/socket.io", RawQuery: "room=room1"}
	log.Println("Connecting to ", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("Received: %s", message)
		}
	}()

	msg := map[string]interface{}{
		"userId": "572385732857345",
		"tripId": "1234567890",
	}
	e := sendEvent(c, "identifyUser", msg)
	if e != nil {
		log.Println("Error sending event")
	}
	// Send a message to broadcast to the room
	// err = c.WriteJSON(msg)
	// if err != nil {
	// 	log.Println("write:", err)
	// 	return
	// }

	// Keep the client running
	select {}
}

func sendEvent(con *websocket.Conn, eventName string, message map[string]interface{}) error {
	msg := message
	msg["action"] = eventName

	err := con.WriteJSON(msg)
	if err != nil {
		log.Println("write:", err)
		return err
	}
	log.Println("Sent Event: ", eventName, " Data: ", message)
	return nil

}
