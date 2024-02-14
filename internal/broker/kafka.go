package broker

import "sync"

type Kafka struct {
}

var (
	kOnce         sync.Once
	kafkaInstance *Kafka
)

func (k *Kafka) GetInstance() *Kafka {
	kOnce.Do(func() {
		kafkaInstance = &Kafka{}
	})
	return kafkaInstance
}

func (k *Kafka) PublishMessage() {

}

func (k *Kafka) Connect(con string) error {
	return nil
}
