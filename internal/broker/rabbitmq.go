package broker

import (
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	conn *amqp091.Connection
	ch   *amqp091.Channel
}

var (
	rOnce            sync.Once
	rabbitMqInstance *RabbitMQ
)

func (r *RabbitMQ) PublishMessage(data interface{}) {
	return
}

// func (r *RabbitMQ) GetInstance() *RabbitMQ {
// 	rOnce.Do(func() {
// 		//do the connect thing here
// 		r.ConnectBroker()
// 	})
// 	return r
// }

func (r *RabbitMQ) ConnectBroker() error {
	godotenv.Load()
	rHost := os.Getenv("RABBITMQ_HOST")
	rUser := os.Getenv("RABBITMQ_USERNAME")
	rPass := os.Getenv("RABBITMQ_PASSWORD")
	rPort := os.Getenv("RABBITMQ_PORT")
	connString := "amqp://" + rUser + ":" + rPass + "@" + rHost + ":" + rPort + "/"
	rabbitConn, connErr := amqp091.Dial(connString)
	if connErr != nil {
		return connErr
	}
	r.conn = rabbitConn
	r.ch, connErr = rabbitConn.Channel()
	if connErr != nil {
		return connErr
	}
	log.Println("Connected to RabbitMQ", r.conn, r.ch)
	defer r.conn.Close()

	return nil
}
