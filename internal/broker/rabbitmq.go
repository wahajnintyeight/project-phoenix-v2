package broker

import (
	"sync"

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

func (r *RabbitMQ) PublishMessage() {
}

func (r *RabbitMQ) GetInstance() *RabbitMQ {
	rOnce.Do(func() {
		rabbitMqInstance = &RabbitMQ{}
	})
	return rabbitMqInstance
}

func (r *RabbitMQ) Connect(con string) error {
	return nil
}
