package service

import (
	internal "project-phoenix/v2/internal/service-configs"

	"go-micro.dev/v4"
	microBroker "go-micro.dev/v4/broker"

)

type ServiceInterface interface {
	Start() error
	Stop() error
	InitializeService(micro.Service, string) (ServiceInterface)
	GetSubscribedTopics() ([]internal.SubscribedTopicsMap)
	ListenSubscribedTopics(microBroker.Event) error
	SubscribeTopics()
	InitServiceConfig()
}
