package factory

import (
	"log"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/pkg/service"

    apiGateway	"project-phoenix/v2/pkg/service/apigateway"
	dataCommunicator "project-phoenix/v2/pkg/service/datacommunicator"
	locationService "project-phoenix/v2/pkg/service/locationservice"

	"go-micro.dev/v4"
)

func ServiceFactory(serviceObj micro.Service, serviceType enum.ServiceType, serviceName string) service.ServiceInterface {
	log.Print("Service Factory ", serviceName, " ", serviceType)
	switch serviceType {
	case enum.APIGateway:
		apiGatewayService := apiGateway.NewAPIGatewayService(serviceObj, serviceName)
		return apiGatewayService
	case enum.Location:
		locationService := locationService.NewLocationService(serviceObj, serviceName)
		return locationService
	case enum.DataCommunicator:
		dataCommunicatorService := dataCommunicator.NewDataCommunicatorService(serviceObj, serviceName)
		return dataCommunicatorService
	default:
		panic("Invalid service type")
	}
}
