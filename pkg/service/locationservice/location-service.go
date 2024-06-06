package service

import (
	"encoding/json"
	"fmt"
	"log"
	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	internal "project-phoenix/v2/internal/service-configs"
	"reflect"
	"strconv"
	"sync"
	"time"

	"project-phoenix/v2/pkg/helper"
	"project-phoenix/v2/pkg/service"

	"github.com/go-micro/plugins/v4/broker/rabbitmq"
	microBroker "go-micro.dev/v4/broker"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"go-micro.dev/v4"
)

type LocationService struct {
	service            micro.Service
	subscribedServices []internal.SubscribedServices
	serviceConfig      internal.ServiceConfig
	brokerObj          microBroker.Broker
}

// var locationServiceObj *LocationService

const (
	MaxRetries  = 6
	RetryDelay  = 2 * time.Second
	serviceName = "location-service"
)

var locationOnce sync.Once

func (ls *LocationService) GetSubscribedTopics() []internal.SubscribedServices {
	serviceConfig, e := internal.ReturnServiceConfig(serviceName)
	if e != nil {
		log.Println("Unable to read service config", e)
		return nil
	}
	ls.subscribedServices = serviceConfig.(*internal.ServiceConfig).SubscribedServices
	return ls.subscribedServices
}

func (ls *LocationService) InitServiceConfig() {
	serviceConfig, er := internal.ReturnServiceConfig(serviceName)
	if er != nil {
		log.Println("Unable to read service config", er)
		return
	}
	ls.serviceConfig = serviceConfig.(internal.ServiceConfig)
}

func (ls *LocationService) SubscribeTopics() {
	ls.InitServiceConfig()
	for _, service := range ls.serviceConfig.SubscribedServices {
		log.Println("Service", service)
		for _, topic := range service.SubscribedTopics {
			log.Println("Preparing to subscribe to service: ", service.Name, " | Topic: ", topic.TopicName, " | Queue: ", service.Queue, " | Handler: ", topic.TopicHandler, " | MaxRetries: ", MaxRetries)
			if err := ls.attemptSubscribe(service.Queue, topic); err != nil {
				log.Printf("Subscription failed for topic %s: %v", topic.TopicName, err)
			}
		}
	}
}

// attemptSubscribe tries to subscribe to a topic with retries until successful or max retries reached.
func (ls *LocationService) attemptSubscribe(queueName string, topic internal.SubscribedTopicsMap) error {
	log.Println("Max Retries:", MaxRetries)
	for i := 0; i <= MaxRetries; i++ {
		log.Println("Attempting to subscribe to", topic, " for Queue", queueName)
		if err := ls.subscribeToTopic(queueName, topic); err != nil {
			if err.Error() == "not connected" && i < MaxRetries {
				log.Printf("Broker not connected, retrying %d/%d for topic %s", i+1, MaxRetries, topic.TopicName)
				time.Sleep(RetryDelay)
				continue
			}
			return err
		}
		break
	}
	return nil
}

func (ls *LocationService) subscribeToTopic(queueName string, topic internal.SubscribedTopicsMap) error {
	handler, ok := reflect.TypeOf(ls).MethodByName(topic.TopicHandler)
	if !ok {
		return fmt.Errorf("Handler method %s not found for topic %s", topic.TopicHandler, topic.TopicName)
	}

	_, err := ls.brokerObj.Subscribe(topic.TopicName, func(p microBroker.Event) error {
		returnValues := handler.Func.Call([]reflect.Value{reflect.ValueOf(ls), reflect.ValueOf(p)})
		if err, ok := returnValues[0].Interface().(error); ok && err != nil {
			return err
		}
		return nil
	}, microBroker.Queue(queueName), rabbitmq.DurableQueue())

	if err != nil {
		log.Printf("Failed to subscribe to topic %s due to error: %v", topic.TopicName, err)
		return err
	}

	log.Printf("Successfully subscribed to topic %s | Handler: %s", topic.TopicName, topic.TopicHandler)
	return nil
}

func (ls *LocationService) ListenSubscribedTopics(broker microBroker.Event) error {
	// ls.brokerObj.Subscribe()
	// broker
	log.Println("Broker Event: ", broker)
	log.Println("Broker Event: ", broker.Message().Header)
	return nil
}

func (ls *LocationService) InitializeService(serviceObj micro.Service, serviceName string) service.ServiceInterface {

	locationOnce.Do(func() {
		service := serviceObj
		ls.service = service
		ls.brokerObj = serviceObj.Options().Broker
		log.Println("Location Service Broker Instance: ", ls.brokerObj)
	})
	return ls
}

func (ls *LocationService) HandleStartTracking(p microBroker.Event) error {
	log.Println("Start Tracking Func Called | Data: ", p.Message().Header, " | Body: ", p.Message().Body)
	data := make(map[string]interface{})
	if err := json.Unmarshal(p.Message().Body, &data); err != nil {
		log.Println("Error occurred while unmarshalling the data", err)
	}
	log.Println("Data Received: ", data)
	return nil
}

func (ls *LocationService) HandleProcessLocation(p microBroker.Event) error {
	log.Println("Process Location Func Called | Data: ", p.Message().Header, " | Body: ", p.Message().Body)

	data := make(map[string]interface{})
	if err := json.Unmarshal(p.Message().Body, &data); err != nil {
		log.Println("Error occurred while unmarshalling the data", err)
		return err
	}
	locationData := model.LocationData{}
	er := helper.InterfaceToStruct(data["data"], &locationData)
	if er != nil {
		log.Println("Error decoding the data map", er)
		return er
	}
	log.Println("Data Received: ", locationData)
	ProcessUserTripLocation(locationData)
	return nil
}

func ProcessUserTripLocation(locationData model.LocationData) error {
	currentDate := time.Now()

	// ctx := context.Background()

	userLocationControllerInstance := controllers.GetControllerInstance(enum.UserLocationController, enum.MONGODB).(*controllers.UserLocationController)
	// mdb, er := db.GetDBInstance(enum.MONGODB)
	// if er != nil {

	// } else {
	// 	// mdb.
	// }
	query := map[string]interface{}{
		"userId": locationData.UserId,
		"tripId": locationData.TripId,
	}
	result, e := userLocationControllerInstance.FindLastDocument(query)
	if e != nil {
		log.Println("Error occurred while fetching the last document", e)
		return e
	}
	log.Println("Result: ", result)
	isNewDay := false
	locationDataModel := &model.UserLocation{}
	jsonE := helper.InterfaceToStruct(result, &locationDataModel)
	if jsonE != nil {
		return jsonE
	}
	log.Println("Current Date | Location Date: ", currentDate.Day(), locationDataModel.CreatedAt.Day())
	if locationDataModel.CreatedAt.Day() != currentDate.Day() {
		isNewDay = true
	} else {
		isNewDay = false
	}
	lat := strconv.FormatFloat(locationData.CurrentLat, 'f', -1, 64)
	lng := strconv.FormatFloat(locationData.CurrentLng, 'f', -1, 64)
	if isNewDay {
		log.Println("Attempting to insert a new location for the day")
		newLocation := model.UserLocation{
			UserID:     locationData.UserId,
			TripID:     locationData.TripId,
			CurrentLat: lat,
			CurrentLng: lng,
			StartLat:   lat,
			StartLng:   lng,
			IsStarted:  true,
			CreatedAt:  currentDate,
			UpdatedAt:  currentDate,
			LastLat:    lat,
			LastLng:    lng,
		}
		d, e := userLocationControllerInstance.Create(newLocation)
		if e != nil {
			log.Println("Error inserting a new location", e)
			return e
		} else {
			log.Println("Inserte a new location", d)
		}

	} else {
		updateLocationDataMap := map[string]interface{}{
			"tripId":     locationData.TripId,
			"userId":     locationData.UserId,
			"currentLat": lat,
			"currentLng": lng,
			"updatedAt" : currentDate,
		}

		//merge a key value pair to query map interface
		locationDataID , ero := primitive.ObjectIDFromHex(locationDataModel.ID) 
		if ero != nil {
			panic(ero)
		}
		query["_id"] = locationDataID
		d, e := userLocationControllerInstance.CreateOrUpdate(query, updateLocationDataMap)
		if e != nil {
			log.Println("Error creating or updating user location", e)
			return e
		} else {
			log.Println("Update the location data. ", d)
		}
	}
	return nil

}

func (ls *LocationService) HandleStopTracking(p microBroker.Event) error {
	log.Println("Stop Tracking Func Called | Data: ", p.Message().Header, " | Body: ", p.Message().Body)
	data := make(map[string]interface{})
	if err := json.Unmarshal(p.Message().Body, &data); err != nil {
		log.Println("Error occurred while unmarshalling the data", err)
	}
	log.Println("Data Received: ", data)
	return nil
}

func NewLocationService(serviceObj micro.Service, serviceName string) service.ServiceInterface {
	locationService := &LocationService{}
	return locationService.InitializeService(serviceObj, serviceName)
}

func (ls *LocationService) Start(port string) error {
	log.Print("Location Service Started on Port:", port)
	ls.SubscribeTopics()
	return nil
}

func (ls *LocationService) Stop() error {
	return nil
}
