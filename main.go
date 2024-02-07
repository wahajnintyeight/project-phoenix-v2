package main

import (
	"flag"
	"fmt"
	"os"
	"project-phoenix/v2/pkg/factory"
)

func main() {
	serviceTypeFlag := flag.String("s", "", "Name of the Service")
	portFlag := flag.Int("p", 0, "The port on which the service will be running")

	flag.Parse()

	if *serviceTypeFlag == "" || *portFlag == 0 {
		fmt.Println("Error occurred: Service type and port are required")
		fmt.Println("Usage: go run main.go -s=<service-name> -p=<port>")
		os.Exit(1)
	}

	var serviceType factory.ServiceType

	switch *serviceTypeFlag {
	case "api-gateway":
		serviceType = factory.APIGateway
		break
	case "location-service":
		serviceType = factory.Location
		break
	default:
		fmt.Println("Error occurred: Invalid service type")

	}

	serviceObj := factory.ServiceFactory(serviceType, *serviceTypeFlag)

	if err := serviceObj.Start(); err != nil {
		fmt.Println("Failed to start the service: %v", err)
	}
}
