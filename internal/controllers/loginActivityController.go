package controllers

import (
	"project-phoenix/v2/internal/db"

	"go.mongodb.org/mongo-driver/bson"
)

type LoginActivityController struct {
	CollectionName string
	DB             db.DBInterface
}

func (la *LoginActivityController) GetCollectionName() string {
	return "loginactivities"
}

func (la *LoginActivityController) PerformIndexing() error {
	// indexes := []interface{}{"sessionId", "createdAt", "token", "userId", "email"}
	indexes := []bson.D{
		{{Key: "sessionId", Value: 1}},
		{{Key: "createdAt", Value: 1}},
		{{Key: "token", Value: 1}},
		{{Key: "userId", Value: 1}},
		{{Key: "email", Value: 1}},
	}
	var validateErr error
	minutes := 120 //2 hours
	for _, index := range indexes {
		validateErr = la.DB.ValidateIndexingTTL(la.GetCollectionName(), index, minutes)
		if validateErr != nil {
			return validateErr
		}
	}
	return nil

}
