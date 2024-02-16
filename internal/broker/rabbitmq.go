package broker

import (
	"log"
	"os"
	"project-phoenix/v2/pkg/helper"
	"sync"

	"github.com/joho/godotenv"
	"github.com/rabbitmq/amqp091-go"
	"go-micro.dev/v4/broker"
)

type RabbitMQ struct {
	conn *amqp091.Connection
	// ch   *amqp091.Channel
	rabbitMQBroker broker.Broker
}

var (
	rOnce            sync.Once
	rabbitMqInstance *RabbitMQ
)

func (r *RabbitMQ) PublishMessage(data map[string]interface{}, serviceName string, topicName string) {
	byteData, e := helper.MarshalBinary(data)
	if e != nil {
		log.Println("Error occurred while marshalling the data", e)
		return
	}
	message := &broker.Message{
		Header: map[string]string{
			"service": serviceName,
		},
		Body: byteData,
	}
	if pubErr := broker.Publish(topicName, message); pubErr != nil {
		log.Println("Unable to publish message to: ", topicName, " Error: ", pubErr)
		return
	}
	return
}

// func (r *RabbitMQ) GetInstance() *RabbitMQ {
// 	rOnce.Do(func() {
// 		//do the connect thing here
// 		r.ConnectBroker()
// 	})
// 	return r
// }

func (r *RabbitMQ) SubscribeTopic (){
	
}

func (r *RabbitMQ) ConnectBroker() error {
	godotenv.Load()
	rHost := os.Getenv("RABBITMQ_HOST")
	rUser := os.Getenv("RABBITMQ_USERNAME")
	rPass := os.Getenv("RABBITMQ_PASSWORD")
	rPort := os.Getenv("RABBITMQ_PORT")
	connString := "amqp://" + rUser + ":" + rPass + "@" + rHost + ":" + rPort + "/"
	r.rabbitMQBroker = broker.NewBroker(
		// Set the broker to RabbitMQ
		broker.Addrs(connString),
	)
	if e := r.rabbitMQBroker.Connect(); e != nil {
		log.Println("Error occurred while connecting to RabbitMQ", e)
		return e
	}

	defer r.rabbitMQBroker.Disconnect()

	return nil
}
