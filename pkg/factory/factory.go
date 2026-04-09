package factory

import (
	"log"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/pkg/service"

	apiGateway "project-phoenix/v2/pkg/service/apigateway"
	apiGatewayGRPC "project-phoenix/v2/pkg/service/apigateway-grpc"
	dataCommunicator "project-phoenix/v2/pkg/service/datacommunicator"
	locationService "project-phoenix/v2/pkg/service/locationservice"
	scraperService "project-phoenix/v2/pkg/service/scraper-service"
	socketService "project-phoenix/v2/pkg/service/socketservice"
	sseService "project-phoenix/v2/pkg/service/sse-service"
	workerService "project-phoenix/v2/pkg/service/worker-service"

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
	case enum.SocketService:
		socketService := socketService.NewSocketService(serviceObj, serviceName)
		return socketService
	case enum.APIGatewayGRPC:
		apiGatewayGRPCService := apiGatewayGRPC.NewAPIGatewayGRPCService(serviceObj, serviceName)
		return apiGatewayGRPCService
	case enum.SSEService:
		sseService := sseService.NewSSEService(serviceObj, serviceName)
		return sseService
	case enum.WorkerService:
		workerService := workerService.NewWorkerService(serviceObj, serviceName)
		return workerService
	case enum.ScraperService:
		scraperService := scraperService.NewScraperService(serviceObj, serviceName)
		return scraperService
	default:
		panic("Invalid service type")
	}
}
