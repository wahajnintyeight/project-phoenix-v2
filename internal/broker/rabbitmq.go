package broker

import (
	"log"
	"os"
	"project-phoenix/v2/pkg/helper"
	"sync"

	"github.com/go-micro/plugins/v4/broker/rabbitmq"
	"github.com/joho/godotenv"
	amqp091 "github.com/rabbitmq/amqp091-go"
	"go-micro.dev/v4/broker"
)

type RabbitMQ struct {
	conn           *amqp091.Connection
	RabbitMQBroker broker.Broker
	channel        *amqp091.Channel
}

var (
	rOnce            sync.Once
	rabbitMqInstance *RabbitMQ
	channel *amqp091.Channel
	channelOnce sync.Once
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
	if pubErr := r.RabbitMQBroker.Publish(topicName, message); pubErr != nil {
		log.Println("Unable to publish message to: ", topicName, " Error: ", pubErr)
		return
	} else {
		log.Println("Broker Instance | Publish", &r.RabbitMQBroker)
		log.Println("Message published to: ", topicName, " | Data: ", data, " | Service: ", serviceName)
	}

}

func (r *RabbitMQ) SubscribeTopic() {

}

func (r *RabbitMQ) ConnectBroker() error {

	var connErr error
	connErr = nil
	rOnce.Do(func() {
		rabbitMQConnString := ReturnRabbitMQConnString()
		r.RabbitMQBroker = rabbitmq.NewBroker(
			broker.Addrs(rabbitMQConnString),
		)
		if initEr := r.RabbitMQBroker.Init(); initEr != nil {
			log.Println(initEr)
			connErr = initEr
		}
		if e := r.RabbitMQBroker.Connect(); e != nil {
			log.Println(e)
			connErr = e
		} else {
			log.Println("RabbitMQ Connected")
		}
	})
	if connErr != nil {
		return connErr
	} else {
		return nil
	}
}

func ReturnRabbitMQConnString() string {
	godotenv.Load()
	rHost := os.Getenv("RABBITMQ_HOST")
	rUser := os.Getenv("RABBITMQ_USERNAME")
	rPass := os.Getenv("RABBITMQ_PASSWORD")
	rPort := os.Getenv("RABBITMQ_PORT")
	connString := "amqp://" + rUser + ":" + rPass + "@" + rHost + ":" + rPort + "/"
	return connString
}

// Close closes the RabbitMQ connection and channel
func (r *RabbitMQ) Close() {
	if r.channel != nil {
		log.Println("r.channel | Closing RabbitMQ Channel")
		r.channel.Close()
	}
	if r.conn != nil {
		log.Println("r.conn | Closing RabbitMQ Connection")
		r.conn.Close()
	}
}