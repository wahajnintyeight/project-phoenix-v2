package internal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
)

type ServiceConfig struct {
	Port                   string               `json:"port`
	ServiceName            string               `json:"serviceName"`
	ServiceExchange        string               `json:"serviceExchange"`
	ServiceQueue           string               `json:"serviceQueue"`
	EndpointPrefix         string               `json:"endpointPrefix"`
	SessionIDMiddlewareKey string               `json:"sessionIdMiddleware"`
	SubscribedServices     []SubscribedServices `json:"subscribedServices"`
}

type SubscribedServices struct {
	Name             string                `json:"name"`
	Exchange         string                `json:"exchange"`
	Queue            string                `json:"queue"`
	SubscribedTopics []SubscribedTopicsMap `json:"subscribedTopics"`
}
type SubscribedTopicsMap struct {
	TopicName    string `json:"topicName"`
	TopicHandler string `json:"topicHandler"`
}

var serviceConfigObj ServiceConfig

func ReturnServiceConfig(serviceName string) (interface{}, error) {
	//read the path, and return. the file is in json format
	// serviceConfig, err := os.Open("internal/service-configs/" + serviceName + "/service-config.json")

	// if err != nil {
	// 	fmt.Println("Unable to read file",err)
	// 	return nil, err
	// }
	// defer serviceConfig.Close()

	// byteValue, _ := ioutil.ReadAll(serviceConfig)
	// if err := json.Unmarshal(byteValue, &serviceConfigObj); err != nil {
	// 	fmt.Println("Error parsing JSON:", err)
	// 	return nil, err
	// }
	filePath := "internal/service-configs/" + serviceName + "/service-config.json"
	byteValue, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Println("Unable to read file", err)
		return nil, err
	}

	// Unmarshal JSON
	// var serviceConfigObj interface{}
	if err := json.Unmarshal(byteValue, &serviceConfigObj); err != nil {
		fmt.Println("Error parsing JSON:", err)
		return nil, err
	}

	log.Println("Service config obj", serviceConfigObj)

	return serviceConfigObj, nil
}
