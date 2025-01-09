package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	Client *mongo.Client
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
func (m *MongoDB) StartSession() (mongo.Session, error) {
	return m.Client.StartSession(options.Session())
}
func (m *MongoDB) GetInstance() (*MongoDB, error) {
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
				Client: client,
				db:     db,
			}
			m = dbInstance

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
	err := m.Client.Disconnect(context.Background())
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

func (m *MongoDB) CreateWithTTL(data interface{}, collectionName string, ttlMinutes int) (bson.M, error) {
	conn := GetConnectionFromPool()
	defer ReleaseConnectionToPool(conn)

	log.Println("MongoDB | Create With TTL | Data: ", data, " | Collection: ", collectionName)
	collection := conn.db.Collection(collectionName)
	index := mongo.IndexModel{
		Keys:    bson.D{{Key: "createdAt", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(int32(ttlMinutes) * 60),
	}
	_, createErr := collection.Indexes().CreateOne(context.Background(), index)
	if createErr != nil {
		log.Println("Error while creating index: ", createErr)
		return nil, createErr
	}
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

func (m *MongoDB) FindAllWithPagination(query interface{}, page int, collectionName string) (int64, int, []bson.M, error) {
	conn := GetConnectionFromPool()
	defer ReleaseConnectionToPool(conn)
	collection := conn.db.Collection(collectionName)
	const pageSize = 10
	if page < 1 {
		page = 1
	}
	// Calculate the total number of documents
	totalDocs, err := collection.CountDocuments(context.Background(), query)
	if err != nil {
		log.Println(err)
		return 0, 0, nil, err
	}

	// Calculate total pages
	totalPages := totalDocs / pageSize
	if totalDocs%pageSize > 0 {
		totalPages++
	}

	// Fetch documents with pagination
	opts := options.Find().SetLimit(pageSize).SetSkip(pageSize * int64(page-1))
	cursor, err := collection.Find(context.Background(), query, opts)
	if err != nil {
		log.Println(err)
		return 0, 0, nil, err
	}
	defer cursor.Close(context.Background())

	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		log.Println(err)
		return 0, 0, nil, err
	}

	return totalPages, page, results, nil

}

func (m *MongoDB) Update(query interface{}, update interface{}, collectionName string) (string, error) {
	conn := GetConnectionFromPool()
	defer ReleaseConnectionToPool(conn)
	collection := conn.db.Collection(collectionName)
	updateData := bson.M{"$set": update}

	res, e := collection.UpdateOne(context.Background(), query, updateData)
	if e != nil {
		return "", e
	}
	log.Println("MongoDB | Update | Query: ", query, " | Collection: ", collectionName, " | Data: ", updateData)
	return strconv.Itoa(int(res.ModifiedCount)), nil
}

func (m *MongoDB) UpdateAndIncrement(query interface{}, update bson.M, incMap interface{}, setData bson.M, collectionName string) (string, error) {
	conn := GetConnectionFromPool()
	defer ReleaseConnectionToPool(conn)
	collection := conn.db.Collection(collectionName)
	updateData := bson.M{"$push": update, "$inc": incMap, "$set": setData}

	log.Println("MongoDB | Update | Query: ", query, " | Collection: ", collectionName, " | Data: ", updateData)
	res, e := collection.UpdateOne(context.Background(), query, updateData)
	if e != nil {
		return "", e
	}
	return strconv.Itoa(int(res.ModifiedCount)), nil
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

func (m *MongoDB) Delete(data interface{}, collectionName string) (bool, error) {
	conn := GetConnectionFromPool()
	defer ReleaseConnectionToPool(conn)
	collection := conn.db.Collection(collectionName)
	_, err := collection.DeleteOne(context.Background(), data)
	if err != nil {
		log.Println(err)
		return false, err
	} else {
		return true, nil
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

func (m *MongoDB) ValidateIndexingTTL(collectionName string, indexKeys bson.D, ttlMinutes int) error {
	conn := GetConnectionFromPool()
	defer ReleaseConnectionToPool(conn)
	collection := conn.db.Collection(collectionName)
	indexView, err := collection.Indexes().List(context.Background())
	if err != nil {
		log.Println("Error while fetching indexes: ", err)
		return err
	}
	indexName := getIndexName(indexKeys)
	ttlSeconds := int32(ttlMinutes) * 60
	indexModel := mongo.IndexModel{
		Keys:    indexKeys,
		Options: options.Index().SetExpireAfterSeconds(ttlSeconds).SetName(indexName),
	}

	for indexView.Next(context.Background()) {
		var index bson.M
		if err := indexView.Decode(&index); err != nil {
			log.Println("Error while decoding index: ", err)
			return err
		}

		if index["name"] == indexName {
			if index["expireAfterSeconds"] == ttlSeconds {
				return nil
			} else {
				log.Println("Existing index has a different TTL, dropping it and creating a new one.")
				_, dropErr := collection.Indexes().DropOne(context.Background(), indexName)
				if dropErr != nil {
					log.Println("Error while dropping existing index: ", dropErr)
					return dropErr
				}
				break
			}
		}
	}

	_, createErr := collection.Indexes().CreateOne(context.Background(), indexModel)
	if createErr != nil {
		log.Println("Error while creating index: ", createErr)
		return createErr
	}

	return nil
}

func getIndexName(indexKeys bson.D) string {
	var name string
	for _, key := range indexKeys {
		if name != "" {
			name += "_"
		}
		name += key.Key + "_" + fmt.Sprint(key.Value)
	}
	return name
}
