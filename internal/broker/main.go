package broker

import (
	"log"

	// "project-phoenix/v2/internal/broker"
	"project-phoenix/v2/internal/enum"
	"sync"
)

type Broker interface {
	PublishMessage(map[string]interface{}, string, string)
	ConnectBroker() error
	// GetInstance()
}

var (
	rabbitOnce       sync.Once
	kafkaOnce        sync.Once
	rabbitMQInstance *RabbitMQ
	kafkaInstance    *Kafka
)

func CreateBroker(brokerType enum.BrokerType) Broker {
	switch brokerType {
	case enum.RABBITMQ:
		rabbitOnce.Do(func() {
			broker := &RabbitMQ{}
			broker.ConnectBroker()
			rabbitMQInstance = broker
			log.Println("RabbitMQ Instance Created")
		})
		return rabbitMQInstance
	case enum.KAFKA:
		kafkaOnce.Do(func() {
			broker := &Kafka{}
			// Initialize Kafka connection here if necessary
			kafkaInstance = broker
		})
		return kafkaInstance
	default:
		return nil
	}
}

func ReturnBrokerConnectionString(brokerType enum.BrokerType) string {
	switch brokerType {
	case enum.RABBITMQ:
		// broker := &RabbitMQ{}
		return ReturnRabbitMQConnString()
	case enum.KAFKA:
		return "host:port"
	default:
		return ""
	}
}
