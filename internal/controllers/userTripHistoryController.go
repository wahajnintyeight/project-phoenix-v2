package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/helper"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserTripHistoryController struct {
	CollectionName          string
	DB                      db.DBInterface
	APIGatewayServiceConfig internal.ServiceConfig
}

func (sc *UserTripHistoryController) GetCollectionName() string {
	return "usertriphistories"
}

func (sc *UserTripHistoryController) CreateTrip(w http.ResponseWriter, r *http.Request) (int, string, interface{}, error) {
	createTripRequestModel := model.CreateTripModel{}

	decodeErr := json.NewDecoder(r.Body).Decode(&createTripRequestModel)
	if decodeErr != nil {
		log.Println("Error decoding request body", decodeErr)
		return int(enum.ERROR), "Error decoding request body", nil, decodeErr
	} else {
		userTripModel := model.UserTrip{}
		userTripModel.Name = createTripRequestModel.TripName
		userTripModel.CreatedAt = helper.GetCurrentTime()
		userTripModel.UpdatedAt = helper.GetCurrentTime()
		userTripModel.IsNotificationsEnabled = createTripRequestModel.EnableNotification
		userTripModel.TripID = helper.GenerateTripID()
		userTripModel.UserID = helper.GetCurrentUser(r)
		addedTrip, err := sc.DB.Create(userTripModel, sc.GetCollectionName())
		userTripModel.ID = helper.InterfaceToString(addedTrip["_id"])
		if err != nil {
			log.Println("Error creating trip", err)
			return int(enum.ERROR), "Error creating trip", nil, err
		}
		return int(enum.TRIP_CREATED), "Trip Created", userTripModel, nil
	}
}

func (sc *UserTripHistoryController) DeleteTrip(w http.ResponseWriter, r *http.Request) (int, map[string]interface{}, error) {
	deleteTripModel := model.DeleteTripModel{}
	decodeErr := json.NewDecoder(r.Body).Decode(&deleteTripModel)
	if decodeErr != nil {
		log.Println("Error while decoding delete trip model", decodeErr)
		return int(enum.ERROR), nil, decodeErr
	} else {
		query := map[string]interface{}{
			"tripId": deleteTripModel.TripId,
		}
		isDeleted, err := sc.DB.Delete(query, sc.GetCollectionName())
		if err != nil {
			log.Println("Error while deleting", err)
			return int(enum.TRIP_NOT_DELETED), nil, err
		} else {
			log.Println(isDeleted)
			return int(enum.TRIP_DELETED), nil, nil
		}
	}
}
func (sc *UserTripHistoryController) ListAllTrips(w http.ResponseWriter, r *http.Request) (int, map[string]interface{}, error) {
	userID := helper.GetCurrentUser(r)
	page := helper.StringToInt(r.URL.Query().Get("page"))
	query := map[string]interface{}{
		"userId": userID,
	}
	totlaPages, currentPage, result, err := sc.DB.FindAllWithPagination(query, (page), sc.GetCollectionName())
	if err != nil {
		log.Println("Error getting trips", err)
		return int(enum.ERROR), nil, err
	}
	if result == nil {
		result = []primitive.M{}
	}
	return int(enum.TRIP_FOUND), map[string]interface{}{
		"totalPages": totlaPages,
		"page":       currentPage,
		"trips":      result,
	}, nil
}

func (ul *UserTripHistoryController) PerformIndexing() error {
	indexes := []interface{}{"userLocationId", "tripId", "userId"}
	var validateErr error
	for _, index := range indexes {
		validateErr = ul.DB.ValidateIndexing(ul.GetCollectionName(), index)
		if validateErr != nil {
			return validateErr
		}
	}
	return nil
}
