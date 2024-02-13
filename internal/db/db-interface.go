package db

import (
	"errors"
	"log"
	"project-phoenix/v2/internal/enum"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
)

var (
	once sync.Once
)

type DBInterface interface {
	Connect(string) (string, error)
	Disconnect() (string, error)
	Create(interface{}, string) (bson.M, error)
	FindOne(interface{}, string) (bson.M, error)
	FindAll(interface{}, string) (string, error)
	Update(interface{}, interface{}, string) (string, error)
	Delete(interface{}, string) (string, error)
}

func GetDBInstance(dbType enum.DBType) (DBInterface, error) {
	switch dbType {
	case enum.MONGODB:
		instance, err := GetInstance()
		log.Println("DBInterface | GetDBInstance | DB Instance: ", instance, err)
		if err != nil || (err == nil && instance == nil) {
			log.Println("DBInterface | Error while getting DB Instance: ", err)
			return nil, err
		} else {
			return instance, nil
		}
		// Add case for "postgresql" once you have the implementation
	default:
		return nil, errors.New("unknown database type")
	}
}
