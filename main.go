package main

import (
	// "flag"
	"fmt"
	"log"
	"os"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/pkg/factory"

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
			if err := serviceObj.Start(); err != nil {
				log.Fatal(err)
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
