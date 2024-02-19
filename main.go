package main

import (
	// "flag"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/pkg/factory"
	"syscall"

	"github.com/urfave/cli/v2"
	"go-micro.dev/v4"
)

func main() {

	app := &cli.App{
		Name:  "project-phoenix",
		Usage: "A go-micro service",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "service-name",
				Usage:    "Name of the Service",
				Required: true,
			},
			&cli.IntFlag{
				Name:     "port",
				Usage:    "The port on which the service will be running",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			serviceTypeFlag := c.String("service-name")
			portFlag := c.Int("port")
			log.Println(c)
			log.Println("Service Name: ", serviceTypeFlag)

			ctx, _ := context.WithCancel(context.Background())

			service := micro.NewService(
				micro.Name(serviceTypeFlag),
				micro.Address(fmt.Sprintf(":%d", portFlag)),
				micro.Flags(&cli.StringFlag{
					Name:  "service-name",
					Usage: "Name of the service",
				}),
				micro.Flags(&cli.IntFlag{
					Name:  "port",
					Usage: "The port on which the service will be running",
				}),
				micro.Context(ctx),
				// using rabbitmq for now
				// micro.Broker(rabbitmq.NewBroker(
				// 	broker.Addrs(brokerConnString),
				// )),
			)

			// Initialize the service
			service.Init()

			var serviceType enum.ServiceType
			switch serviceTypeFlag {
			case "api-gateway":
				serviceType = enum.APIGateway
			case "location-service":
				serviceType = enum.Location
			default:
				fmt.Println("Error occurred: Invalid service type")
				os.Exit(1)
			}

			go func() {
				// Start Go-Micro service in a goroutine to avoid blocking
				if err := service.Run(); err != nil {
					log.Fatalf("Go-Micro service encountered an error: %v", err)
				}
			}()

			serviceObj := factory.ServiceFactory(service, serviceType, serviceTypeFlag)
			if err := serviceObj.Start(); err != nil {
				log.Fatal("Service start error:", err)
			}

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			<-sigChan // Block until a signal is received for graceful shutdown
			log.Println("Received shutdown signal, shutting down services...")

			// First, gracefully stop the API Gateway or HTTP service
			if err := serviceObj.Stop(); err != nil {
				log.Printf("Error during service shutdown: %v", err)
			}

			// Then, stop the Go-Micro service
			if err := service.Server().Stop(); err != nil {
				log.Printf("Error during Go-Micro service shutdown: %v", err)
			}
			// go func() {
			// 	<-sigChan // Block until a signal is received
			// 	log.Println("Received shutdown signal, shutting down service...")
			// 	if err := service.Server().Stop(); err != nil {
			// 		log.Printf("Error during service shutdown: %v", err)
			// 	}
			// 	cancel()
			// 	log.Println("Service stopped successfully")
			// 	os.Exit(0)
			// }()
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
