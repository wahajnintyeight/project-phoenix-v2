package db

import (
	"errors"
	"log"
	"project-phoenix/v2/internal/enum"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	once            sync.Once
	mongoInstance   *MongoDB
	postgreInstance *PostgreDB
)

type DBInterface interface {
	Connect(string) (string, error)
	Disconnect() (string, error)
	Create(interface{}, string) (bson.M, error)
	FindOne(interface{}, string) (bson.M, error)
	// FindAll(interface{}, string) (string, error)
	Update(interface{}, interface{}, string) (string, error)
	UpdateAndIncrement(interface{}, bson.M, interface{}, bson.M, string) (string, error)
	Delete(interface{}, string) (bool, error)
	CreateWithTTL(interface{}, string, int) (bson.M, error)
	//FindOneAndUpdate finds a single document and updates it, returning either the original or the updated document.
	UpdateOrCreate(interface{}, interface{}, string) interface{}
	ValidateIndexing(string, interface{}) error
	ValidateIndexingTTL(string,bson.D,int) error
	//Fetches the single most recent document from the collection based on the query.
	FindRecentDocument(query interface{}, collectionName string) (interface{}, error)
	FindAllWithPagination(interface{}, int, string) (int64, int, []bson.M, error)
	StartSession() (mongo.Session, error)
}

func GetDBInstance(dbType enum.DBType) (DBInterface, error) {
	switch dbType {
	case enum.MONGODB:
		instance, err := mongoInstance.GetInstance()
		log.Println("DBInterface | GetDBInstance | DB Instance: ", instance, err)
		if err != nil || (err == nil && instance == nil) {
			log.Println("DBInterface | Error while getting DB Instance: ", err)
			return nil, err
		} else {
			return instance, nil
		}
	case enum.POSTGRE:
		instance, err := postgreInstance.GetInstance()
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
