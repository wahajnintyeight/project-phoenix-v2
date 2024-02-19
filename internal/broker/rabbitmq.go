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
	conn *amqp091.Connection
	// ch   *amqp091.Channel
	RabbitMQBroker broker.Broker
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

func (r *RabbitMQ) SubscribeTopic() {

}

func (r *RabbitMQ) ConnectBroker() error {

	// connString := fmt.Sprintf("amqp://%s:%s@%s", rUser, rPass, rHost)
	rabbitMQConnString := ReturnRabbitMQConnString()
	// log.Println("Connection string", (connString))
	r.RabbitMQBroker = rabbitmq.NewBroker(
		broker.Addrs(rabbitMQConnString),
	)
	if initEr := r.RabbitMQBroker.Init(); initEr != nil {
		log.Println(initEr)
		return initEr
	}
	if e := r.RabbitMQBroker.Connect(); e != nil {
		log.Println(e)
		return e
	}

	return nil
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
