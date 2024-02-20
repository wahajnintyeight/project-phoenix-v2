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

			serviceObj := factory.ServiceFactory(service, serviceType, serviceTypeFlag)
			go func() {
				if err := service.Run(); err != nil {
					log.Fatalf("Go-Micro Service Encountered an error: %v", err)
				}
			}()

			go func() {
				if err := serviceObj.Start(); err != nil {
					log.Fatal("Service start error:", err)
				}
			}()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			<-sigChan
			log.Println("Received shutdown signal, shutting down services...")

			// First, gracefully stop the service
			if err := serviceObj.Stop(); err != nil {
				log.Printf("Error during service shutdown: %v", err)
			} else {
				os.Exit(0)
			}

			// Then, stop the Go-Micro service
			if err := service.Server().Stop(); err != nil {
				log.Printf("Error during Go-Micro service shutdown: %v", err)
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
