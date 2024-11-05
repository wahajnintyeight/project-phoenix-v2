package service

import (
	"log"
	"net"
	"sync"

	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/service"
	pb "project-phoenix/v2/pkg/service/apigateway-grpc/src/go"

	"github.com/joho/godotenv"
	"go-micro.dev/v4"
	microBroker "go-micro.dev/v4/broker"
	"google.golang.org/grpc"
)

type APIGatewayGRPCService struct {
	service            micro.Service
	grpcServer        *grpc.Server
	serviceConfig     internal.ServiceConfig
	subscribedServices []internal.SubscribedServices
	brokerObj         microBroker.Broker
	pb.UnimplementedScreenCaptureServiceServer
}

var once sync.Once

func (api *APIGatewayGRPCService) GetSubscribedTopics() []internal.SubscribedServices {
	return nil
}

func (api *APIGatewayGRPCService) InitializeService(serviceObj micro.Service, serviceName string) service.ServiceInterface {
	once.Do(func() {
		service := serviceObj
		api.service = service
		api.grpcServer = grpc.NewServer()
		api.brokerObj = service.Options().Broker
		godotenv.Load()
		servicePath := "api-gateway-grpc"
		serviceConfig, _ := internal.ReturnServiceConfig(servicePath)
		api.serviceConfig = serviceConfig.(internal.ServiceConfig)
		log.Println("GRPC API Service Config", api.serviceConfig)
		log.Println("GRPC API Gateway Service Broker Instance: ", api.brokerObj)

		// Register the gRPC service
		pb.RegisterScreenCaptureServiceServer(api.grpcServer, api)
	})

	return api
}

func (api *APIGatewayGRPCService) ListenSubscribedTopics(broker microBroker.Event) error {
	log.Println("Broker Event: ", broker)
	log.Println("Broker Event: ", broker.Message().Header)
	return nil
}

func (api *APIGatewayGRPCService) SubscribeTopics() {
	api.InitServiceConfig()
}

func (api *APIGatewayGRPCService) InitServiceConfig() {
	serviceConfig, er := internal.ReturnServiceConfig("api-gateway-grpc")
	if er != nil {
		log.Println("Unable to read service config", er)
		return
	}
	api.serviceConfig = serviceConfig.(internal.ServiceConfig)
}

func NewAPIGatewayGRPCService(serviceObj micro.Service, serviceName string) service.ServiceInterface {
	apiGatewayGRPCService := &APIGatewayGRPCService{}
	return apiGatewayGRPCService.InitializeService(serviceObj, serviceName)
}

func (s *APIGatewayGRPCService) Start(port string) error {
	godotenv.Load()

	log.Println("Starting GRPC API Gateway Service on Port:", port)

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	if err := s.grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	return nil
}

func (s *APIGatewayGRPCService) Stop() error {
	log.Println("Stopping GRPC API Gateway")
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	return nil
}
