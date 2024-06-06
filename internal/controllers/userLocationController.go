package controllers

import (
	"log"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/model"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/helper"

	"go.mongodb.org/mongo-driver/bson"
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

func (ul *UserLocationController) CreateOrUpdate(locationParamQuery map[string]interface{}, locationData map[string]interface{}) (interface{}, error) {
	log.Println("Create or Update User Location")
	userLocationQuery := map[string]interface{}{
		"tripId": locationParamQuery["tripId"],
		"userId": locationParamQuery["userId"],
		"_id":    locationParamQuery["_id"],
	}
	data, err := ul.DB.FindRecentDocument(userLocationQuery, ul.GetCollectionName())
	if err != nil {
		return nil, err
	}
	userLocationObj := model.UserLocation{}
	utilsErr := helper.InterfaceToStruct(data, &userLocationObj)
	if utilsErr != nil {
		return nil, utilsErr
	}
	res := ul.DB.UpdateOrCreate(userLocationQuery, locationData, ul.GetCollectionName())
	log.Println("User Location Object: ", userLocationObj)
	return res, nil
}

func (ul *UserLocationController) FindLastDocument(query map[string]interface{}) (interface{}, error) {
	data, e := ul.DB.FindRecentDocument(query, ul.GetCollectionName())
	if e != nil {
		return nil, e
	}
	return data, nil
}

func (ul *UserLocationController) Create(userLocation model.UserLocation) (bson.M, error) {
	d, e := ul.DB.Create(userLocation, ul.GetCollectionName())
	if e != nil {
		log.Println("Error occurred while creating the user location", e)
		return nil, e
	}
	return d, nil
}
