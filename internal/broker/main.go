package broker

import "errors"

type Broker struct {
	PublishMessage (interface{})
	// Connect(string) (error)
	GetInstance (int)
}

type BrokerType int

const (
	RABBITMQ BrokerType = iota
	KAFKA
)

func CreateBroker(brokerType BrokerType) (*Broker, error) {
	switch {
	case RABBITMQ:
		broker := rabbitMqInstance.GetInstance()
		e := broker.Connect("7")
		return broker, nil

	case KAFKA:
		broker := kafkaInstance.GetInstance()
		e := broker.Connect("7")

		return broker, nil
	default:
		return nil, errors.New("unknown broker type")
	}
}
