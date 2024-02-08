package internal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type ServiceConfig struct {
	Port            string `json:"port`
	ServiceName     string `json:"serviceName"`
	ServiceExchange string `json:"serviceExchange"`
	ServiceQueue    string `json:"serviceQueue"`
	EndpointPrefix  string `json:"endpointPrefix"`
}

var serviceConfigObj ServiceConfig

func ReturnServiceConfig(path string) (interface{}, error) {
	//read the path, and return. the file is in json format
	serviceConfig, err := os.Open(path)

	if err != nil {
		fmt.Println("Unable to read file")
		return nil, err
	}
	defer serviceConfig.Close()

	byteValue, _ := ioutil.ReadAll(serviceConfig)
	// var result map[string]interface{}
	json.Unmarshal([]byte(byteValue), &serviceConfigObj)
	return serviceConfigObj, nil
}
