package controllers

import (
	"log"
	"net/http"
	"project-phoenix/v2/internal/broker"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	internal "project-phoenix/v2/internal/service-configs"
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
	// data := map[string]interface{}{
	// 	"some": 2,
	// }
	apiGatewayServiceConfig, err := internal.ReturnServiceConfig("api-gateway")
	if err != nil {
		log.Println("Unable to read service config", err)
		return int(enum.ERROR), "Unable to read service config", nil, err
	}
	// sc.  = apiGatewayServiceConfig.(*internal.ServiceConfig)
	sc.APIGatewayServiceConfig = apiGatewayServiceConfig.(internal.ServiceConfig)
	data := map[string]interface{}{
		"lat": 52.13,
		"lng": 155,
	}
	broker.CreateBroker(enum.RABBITMQ).PublishMessage(data, sc.APIGatewayServiceConfig.ServiceQueue, "start-tracking")
	return int(enum.LOCATION_TRACKING_STARTED), "Tracking Started", nil, nil
}
