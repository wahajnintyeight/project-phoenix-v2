package controllers

import (
	"log"
	"net/http"
	"project-phoenix/v2/internal/broker"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
)

type UserTripController struct {
	CollectionName string
	DB             db.DBInterface
}

func (sc *UserTripController) GetCollectionName() string {
	return "usertrips"
}

func (sc *UserTripController) StartTracking(w http.ResponseWriter, r *http.Request) (int, string, interface{}, error) {

	log.Println("Start Tracking Func Called")
	// data := map[string]interface{}{
	// 	"some": 2,
	// }
	// broker.CreateBroker(enum.RABBITMQ).PublishMessage(data)
}
