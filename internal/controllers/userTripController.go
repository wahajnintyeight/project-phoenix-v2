package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"project-phoenix/v2/internal/broker"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/helper"
)

type UserTripController struct {
	CollectionName          string
	DB                      db.DBInterface
	APIGatewayServiceConfig internal.ServiceConfig
}

func (sc *UserTripController) GetCollectionName() string {
	return "usertrips"
}

func (sc *UserTripController) StartTracking(w http.ResponseWriter, r *http.Request) (int, string, interface{}, error) {

	log.Println("Start Tracking Func Called")
	startTrackingModel := model.StartTrackingModel{}
	decodeErr := json.NewDecoder(r.Body).Decode(&startTrackingModel)
	if decodeErr != nil {
		log.Println("Error decoding request body", decodeErr)
		return int(enum.ERROR), "Error decoding request body", nil, decodeErr
	}
	apiGatewayServiceConfig, err := internal.ReturnServiceConfig("api-gateway")
	if err != nil {
		log.Println("Unable to read service config", err)
		return int(enum.ERROR), "Unable to read service config", nil, err
	}

	controller := GetControllerInstance(enum.UserLocationController, enum.MONGODB)
	userLocationController := controller.(*UserLocationController)
	if userLocationController == nil {
		log.Println("Error getting user location controller")
		return int(enum.ERROR), "Error getting user location controller", nil, nil
	}
	userLocation, err := userLocationController.CreateOrUpdate(startTrackingModel)
	if err != nil {
		log.Println("Error creating or updating user location", err)
		return int(enum.ERROR), "Error creating or updating user location", nil, err
	}
	log.Println("User Location: ", userLocation)

	// sc.  = apiGatewayServiceConfig.(*internal.ServiceConfig)
	sc.APIGatewayServiceConfig = apiGatewayServiceConfig.(internal.ServiceConfig)
	dataInterface, convertErr := helper.StructToMap(startTrackingModel)
	if convertErr != nil {
		log.Println("Error converting struct to map")
		return int(enum.ERROR), "Error converting struct to map", nil, nil
	} else {
		broker.CreateBroker(enum.RABBITMQ).PublishMessage(dataInterface, sc.APIGatewayServiceConfig.ServiceQueue, "start-tracking")
		return int(enum.LOCATION_TRACKING_STARTED), "Tracking Started", nil, nil
	}
}
