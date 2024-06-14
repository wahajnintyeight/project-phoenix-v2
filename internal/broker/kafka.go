package broker

import (
	"log"
	"sync"

	// "github.com/go-micro/plugins/v4/broker/kafka/v1"
	"go-micro.dev/v4/broker"
)

type Kafka struct {
	KafkaObj broker.Broker
}

var (
	kOnce sync.Once
)

func (k *Kafka) GetInstance() *Kafka {
	kOnce.Do(func() {
		k.KafkaObj = broker.NewBroker(
			// broker.Addrs("localhost:9092"),
			// kafka.Options(kafka.NewConfig()),
		)

		if err := k.KafkaObj.Init(); err != nil {
			log.Fatalf("Broker Init error: %v", err)
		}
		if err := k.KafkaObj.Connect(); err != nil {
			log.Fatalf("Broker Connect error: %v", err)
		}
		kafkaInstance = &Kafka{KafkaObj: k.KafkaObj}
	})
	return kafkaInstance
}

func (k *Kafka) PublishMessage(data map[string]interface{}, serviceName string, topicName string) {
	return
}

func (k *Kafka) ConnectBroker() error {
	return nil
}
