package service

import (
	"context"
	"log"
	"net"
	"os"
	"project-phoenix/v2/internal/broker"
	"project-phoenix/v2/internal/enum"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/helper"
	"project-phoenix/v2/pkg/service"
	pb "project-phoenix/v2/pkg/service/apigateway-grpc/src/go"
	"sync"

	"github.com/joho/godotenv"
	"go-micro.dev/v4"
	microBroker "go-micro.dev/v4/broker"
	"google.golang.org/grpc"
)

type APIGatewayGRPCService struct {
	service            micro.Service
	grpcServer         *grpc.Server
	serviceConfig      internal.ServiceConfig
	subscribedServices []internal.SubscribedServices
	brokerObj          microBroker.Broker
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

	lis, err := net.Listen("tcp", ":8880")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterScreenCaptureServiceServer(grpcServer, s)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	return nil
}

func (s *APIGatewayGRPCService) SendCapture(ctx context.Context, req *pb.ScreenCaptureRequest) (*pb.ScreenCaptureResponse, error) {
	// Receive the Device Data sent by Windows Go Client
	imageBlob := helper.BytesToString(req.GetImageData())
	log.Printf("Received screen capture request from client", req.GetOsName(),req.GetDeviceName())

	// Check if directory exists, create if not
	err := os.MkdirAll("output/images", 0755)
	if err != nil {
		log.Println("Error creating directory", err)
		return &pb.ScreenCaptureResponse{
			Success: false,
			Message: "Error creating directory: " + err.Error(),
		}, nil
	}

	// Create file with .txt extension
	file, e := os.Create("output/images/" + req.GetDeviceName() + ".txt") 
	if e != nil {
		log.Println("Error creating new file", e)
		return &pb.ScreenCaptureResponse{
			Success: false,
			Message: "Error creating file: " + e.Error(),
		}, nil
	}
	defer file.Close()

	_, err = file.WriteString(imageBlob)
	if err != nil {
		log.Println("Error writingfile", err)
		return &pb.ScreenCaptureResponse{
			Success: false,
			Message: "Error!" + e.Error(),
		}, nil
	}
	// Add your screen capture handling logic here
	// For example:
	message := map[string]interface{}{
		"data": req,
	}

	broker.CreateBroker(enum.RABBITMQ).PublishMessage(message,"api-gateway-grpc-queue","capture-device-data")

	return &pb.ScreenCaptureResponse{
		Success: true,
		Message: "Screen capture processed successfully",
	}, nil
}

func (s *APIGatewayGRPCService) Stop() error {
	log.Println("Stopping GRPC API Gateway")
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	return nil
}
