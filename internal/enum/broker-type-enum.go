package enum

type BrokerType int

const (
	RABBITMQ BrokerType = iota
	KAFKA
)