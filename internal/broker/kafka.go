package broker

import "sync"

type Kafka struct {
}

var (
	kOnce         sync.Once
	kafkaObj *Kafka
)

// func (k *Kafka) GetInstance() *Kafka {
// 	kOnce.Do(func() {
// 		kafkaObj = &Kafka{}
// 	})
// 	return kafkaInstance
// }

func (k *Kafka) PublishMessage(data map[string]interface{},serviceName string,topicName string) {
	return
}

func (k *Kafka) ConnectBroker() error {
	return nil
}
