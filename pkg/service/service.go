package service

import "go-micro.dev/v4"

type ServiceInterface interface {
	Start() error
	Stop() error
	InitializeService(micro.Service, string) ServiceInterface
}
