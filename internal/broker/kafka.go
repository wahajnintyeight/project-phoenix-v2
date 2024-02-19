package broker

import (
	"sync"

	"go-micro.dev/v4/broker"
)

type Kafka struct {
	KafkaObj broker.Broker
}

var (
	kOnce sync.Once
)

// func (k *Kafka) GetInstance() *Kafka {
// 	kOnce.Do(func() {
// 		kafkaObj = &Kafka{}
// 	})
// 	return kafkaInstance
// }

func (k *Kafka) PublishMessage(data map[string]interface{}, serviceName string, topicName string) {
	return
}

func (k *Kafka) ConnectBroker() error {
	return nil
}
