package factory

import (
	"log"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/pkg/service"

	"go-micro.dev/v4"
)

func ServiceFactory(serviceObj micro.Service, serviceType enum.ServiceType, serviceName string) service.ServiceInterface {
	log.Print("Service Factory ", serviceName, " ", serviceType)
	switch serviceType {
	case enum.APIGateway:
		apiGatewayService := service.NewAPIGatewayService(serviceObj, serviceName)
		return apiGatewayService
	case enum.Location:
		locationService := service.NewLocationService(serviceObj, serviceName)
		return locationService
	case enum.DataCommunicator:
		dataCommunicatorService := service.NewDataCommunicatorService(serviceObj, serviceName)
		return dataCommunicatorService
	default:
		panic("Invalid service type")
	}
}
