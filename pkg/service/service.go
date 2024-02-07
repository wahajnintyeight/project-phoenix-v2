package service

type ServiceInterface interface {
	Start() error
	Stop() error
}
