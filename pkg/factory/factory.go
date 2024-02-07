package factory

import (
	"project-phoenix/v2/pkg/service"
)

type ServiceType int

const (
	// Define constants for each service type
	APIGateway ServiceType = iota
	Location
)

// String provides the string representation of the ServiceType for easier debugging and logging
func (st ServiceType) String() string {
	return [...]string{"APIGateway", "Location"}[st]
}

func ServiceFactory(serviceType ServiceType, serviceName string) service.ServiceInterface {
	switch serviceType {
	case APIGateway:
		return service.InitializeService(serviceName)
	case Location:
		return service.InitializeService(serviceName)
	default:
		panic("Invalid service type")
	}
}
