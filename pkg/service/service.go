package service

import (
	internal "project-phoenix/v2/internal/service-configs"

	"go-micro.dev/v4"
	microBroker "go-micro.dev/v4/broker"

)

type ServiceInterface interface {
	Start(string) error
	Stop() error
	InitializeService(micro.Service, string) (ServiceInterface)
	GetSubscribedTopics() ([]internal.SubscribedServices)
	ListenSubscribedTopics(microBroker.Event) error
	SubscribeTopics()
	InitServiceConfig()
}
