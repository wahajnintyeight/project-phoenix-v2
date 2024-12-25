package service

import (
	"context"

	"fmt"
	"log"
	"net"
	"os"
	"project-phoenix/v2/internal/broker"
	"project-phoenix/v2/internal/enum"
	internal "project-phoenix/v2/internal/service-configs"
 
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
	log.Printf("Received screen capture request from client")

	// publicID := req.GetDeviceName() + "_" + req.GetTimesTamp()
	// imageBlob := helper.BytesToString(req.GetImageBlob())
	// SaveImageToFile(imageBlob, req.GetDeviceName())
	// secureURL, err := UploadImageToCloudinary(ctx, req.GetImageBlob(), publicID, "screen_captures")
	// if err != nil {
	// 	log.Println("Error during Cloudinary upload:", err)
	// 	return &pb.ScreenCaptureResponse{
	// 		Success: false,
	// 		Message: err.Error(),
	// 	}, nil
	// }


	// log.Println("Upload Result: ", secureURL)

	message := map[string]interface{}{
		"data": map[string]interface{}{
			"lastImage":    req.GetLastImage(),
		 	"deviceName":   req.GetDeviceName(),
			"timesTamp":    req.GetTimesTamp(),
			"osName":       req.GetOsName(),
			"memoryUsage":  req.GetMemoryUsage(),
			"diskUsage":    req.GetDiskUsage(),
		},
	}

	broker.CreateBroker(enum.RABBITMQ).PublishMessage(message,"api-gateway-grpc-queue","capture-device-data")

	return &pb.ScreenCaptureResponse{
		Success: true,
		Message: "Screen capture processed successfully",
	}, nil
}

func SaveImageToFile(imageBlob string, deviceName string) error {
	// Create output directory if it doesn't exist
	err := os.MkdirAll("output/images", 0755)
	if err != nil {
		log.Println("Error creating directory", err)
		return fmt.Errorf("error creating directory: %v", err)
	}

	// Create file with .txt extension
	file, err := os.Create("output/images/" + deviceName + ".txt")
	if err != nil {
		log.Println("Error creating new file", err) 
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	_, err = file.WriteString(imageBlob)
	if err != nil {
		log.Println("Error writing file", err)
		return fmt.Errorf("error writing file: %v", err)
	}

	log.Printf("Successfully saved image blob to output/images/%s.txt", deviceName)
	return nil
}


func (s *APIGatewayGRPCService) Stop() error {
	log.Println("Stopping GRPC API Gateway")
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	return nil
}
