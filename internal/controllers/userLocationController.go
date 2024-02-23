package controllers

import (
	"log"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/model"
	internal "project-phoenix/v2/internal/service-configs"
)

type UserLocationController struct {
	CollectionName          string
	DB                      db.DBInterface
	APIGatewayServiceConfig internal.ServiceConfig
}

func (ul *UserLocationController) GetCollectionName() string {
	return "userlocations"
}

func (ul *UserLocationController) PerformIndexing() error {
	indexes := []interface{}{"tripId", "userId"}
	var validateErr error
	for _, index := range indexes {
		validateErr = ul.DB.ValidateIndexing(ul.GetCollectionName(), index)
		if validateErr != nil {
			return validateErr
		}
	}
	return nil

}

func (ul *UserLocationController) CreateOrUpdate(locationParam model.StartTrackingModel) (interface{}, error) {
	log.Println("Create or Update User Location")
	userLocationQuery := map[string]interface{}{
		"tripId": locationParam.TripID,
	}
	data, err := ul.DB.FindRecentDocument(userLocationQuery, ul.GetCollectionName())
	if err != nil {
		return nil, err
	}
	log.Println("Data Location:", data)
	userLocationObj := model.UserLocation{}
	// utilsErr := json.Marshal() //helper.MapToStruct(data, &userLocationObj)
	// if utilsErr != nil {
	// 	return nil, utilsErr
	// }
	return nil, nil
}
