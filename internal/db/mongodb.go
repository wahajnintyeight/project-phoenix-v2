package db

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	client *mongo.Client
	db     *mongo.Database
}

var (
	mongoOnce  sync.Once
	dbInstance *MongoDB
	mongoPool  chan *MongoDB
	poolSize   = 10 // Adjust pool size as needed
)

func init() {
	mongoPool = make(chan *MongoDB, poolSize)
}

func GetInstance() (*MongoDB, error) {
	once.Do(func() {
		// Load the .env file
		log.Println("Initializing MongoDB Connection")
		godotenv.Load()
		mongoURI := os.Getenv("MONGO_URI")
		dbName := os.Getenv("MONGO_DB_NAME")

		clientOptions := options.Client().ApplyURI(mongoURI)
		client, err := mongo.Connect(context.Background(), clientOptions)
		if err != nil {
			log.Println(err)
			return
		} else {

			err = client.Ping(context.Background(), nil)
			if err != nil {
				log.Println("Failed to ping MongoDB: ", err)
			} else {
				log.Println("Initialized MongoDB Connection | DB Name: ", dbName)
			}

			db := client.Database(dbName)
			dbInstance = &MongoDB{
				client: client,
				db:     db,
			}

			for i := 0; i < poolSize; i++ {
				mongoPool <- dbInstance
			}

		}
	})
	return dbInstance, nil

}

func GetConnectionFromPool() *MongoDB {
	return <-mongoPool
}

func ReleaseConnectionToPool(conn *MongoDB) {
	mongoPool <- conn
}

func (m *MongoDB) Connect(uri string) (string, error) {

	return "MongoDB connected", nil
}

func (m *MongoDB) Disconnect() (string, error) {
	err := m.client.Disconnect(context.Background())
	if err != nil {
		log.Println(err)
	}
	return "MongoDB disconnected", nil
}

func (m *MongoDB) Create(data interface{}, collectionName string) (bson.M, error) {

	conn := GetConnectionFromPool()
	defer ReleaseConnectionToPool(conn)

	log.Println("MongoDB | Create | Data: ", data, " | Collection: ", collectionName)
	collection := conn.db.Collection(collectionName)
	result, err := collection.InsertOne(context.Background(), data)
	log.Println("Inserted Result", result)
	if err != nil {
		log.Println(err)
		return nil, err
	} else {
		insertedID := result.InsertedID
		return bson.M{"_id": insertedID}, nil
	}
}

func (m *MongoDB) FindOne(data interface{}, collectionName string) (bson.M, error) {
	conn := GetConnectionFromPool()
	defer ReleaseConnectionToPool(conn)
	log.Println("MongoDB | FindOne | Data: ", data, " | Collection: ", collectionName)
	collection := conn.db.Collection(collectionName)
	filter := data
	var result primitive.M
	err := collection.FindOne(context.Background(), filter).Decode(&result)
	if err != nil {
		log.Println("MongoDB | Unable to find data in ", collectionName, " | Query: ", filter)
		return nil, err
	} else {
		return result, nil
	}
}

func (m *MongoDB) FindAll(data interface{}, collectionName string) (string, error) {
	return "MongoDB find all", nil
}

func (m *MongoDB) Update(data interface{}, update interface{}, collectionName string) (string, error) {
	return "MongoDB update", nil
}

func (m *MongoDB) UpdateOrCreate(query interface{}, update interface{}, collectionName string) interface{} {
	conn := GetConnectionFromPool()
	defer ReleaseConnectionToPool(conn)
	collection := conn.db.Collection(collectionName)
	updateData := bson.M{"$set": update}
	res, err := collection.UpdateOne(context.Background(), query, updateData, options.Update().SetUpsert(true))
	if err != nil {
		log.Printf("MongoDB | Unable to update or create data in %s | Error: %v | Query: %v\n", collectionName, err, query)
		return nil
	}
	if res.UpsertedID == nil {
		existingData, _ := m.FindOne(query, collectionName)
		if existingData != nil {
			return existingData["_id"]
		}
	}
	log.Println("MongoDB | UpdateOrCreate | UpsertedID: ", res.UpsertedID)
	return res.UpsertedID

}

func (m *MongoDB) Delete(data interface{}, collectionName string) (string, error) {
	conn := GetConnectionFromPool()
	defer ReleaseConnectionToPool(conn)
	collection := conn.db.Collection(collectionName)
	_, err := collection.DeleteOne(context.Background(), data)
	if err != nil {
		log.Println(err)
		return "", err
	} else {
		return "Deleted", nil
	}
}

func (m *MongoDB) FindRecentDocument(query interface{}, collectionName string) (interface{}, error) {

	conn := GetConnectionFromPool()
	defer ReleaseConnectionToPool(conn)
	collection := conn.db.Collection(collectionName)
	resultInterface := map[string]interface{}{}
	log.Println("Find Recent Document Query:", query, " | Collection: ", collectionName)
	result := collection.FindOne(
		context.Background(),
		query,
		options.FindOne().SetSort(bson.M{"createdAt": -1}))
	log.Println("Error", result.Err())
	if result.Err() != nil {
		log.Println("Error while finding recent document: ", result.Err())
		return nil, result.Err()
	}
	if err := result.Decode(&resultInterface); err != nil {
		log.Println("Error decoding result: ", err)
		return nil, err
	}
	log.Println("Result: ", resultInterface)
	return resultInterface, nil // Return the actual document found

}

func (m *MongoDB) ValidateIndexing(collectionName string, indexKeys interface{}) error {
	conn := GetConnectionFromPool()
	defer ReleaseConnectionToPool(conn)
	collection := conn.db.Collection(collectionName)
	indexView, e := collection.Indexes().List(context.Background())
	//first check if the index exist
	if e != nil {
		log.Println("Error while fetching indexes: ", e)
		//if there is no index, create them
		indexModel := mongo.IndexModel{
			Keys: indexKeys,
		}
		_, err := collection.Indexes().CreateOne(context.Background(), indexModel)
		if err != nil {
			log.Println("Error while creating index: ", err)
			return err
		}
		return nil
	}
	for indexView.Next(context.Background()) {
		var index bson.M
		indexView.Decode(&index)
		if index["key"] == indexKeys {
			log.Println("Index already exists")
			return nil
		}
	}
	return nil
}
