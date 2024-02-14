package helper

import (
	"encoding/json"
	"log"
	"reflect"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func UnmarshalBinary(data []byte, target interface{}) error {
	err := json.Unmarshal(data, target)
	if err != nil {
		log.Println("Failed to unmarshal JSON: %w", err)
		return err
	}
	return nil
}

// MarshalBinary takes an interface{} as input, attempts to marshal it into a JSON byte slice,
// and returns the byte slice along with any error encountered.
func MarshalBinary(input interface{}) ([]byte, error) {
	data, err := json.Marshal(input)
	if err != nil {
		log.Print("Failed to marshal into JSON: %w", err)
		return nil, err
	}
	return data, nil
}

// convert map to string
func MapToString(data map[string]interface{}) string {
	dataByte, _ := MarshalBinary(data)
	return string(dataByte)
}

// convert interface to string
func InterfaceToString(data interface{}) string {

	if oid, ok := data.(primitive.ObjectID); ok {
		return oid.Hex()
	}

	dataByte, err := json.Marshal(data)
	if err != nil {
		log.Println("Failed to marshal data: ", err)
		return ""
	}
	return string(dataByte)
}

// convert map interface to struct
func MapToStruct(data map[string]interface{}, target interface{}) error {
	dataByte, _ := MarshalBinary(data)
	err := UnmarshalBinary(dataByte, target)
	if err != nil {
		log.Println("Failed to unmarshal data: ", err)
		return err
	}
	return nil
}

func MergeStructAndMap(structData interface{}, additionalData map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	v := reflect.ValueOf(structData)
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue // Skip fields without JSON tags or opted out of JSON serialization
		}

		// Handle json tags with omitempty
		jsonKey := strings.Split(jsonTag, ",")[0]

		// Convert time.Time to a format (e.g., RFC3339) if needed
		if field.Type == reflect.TypeOf(time.Time{}) {
			timeField := v.Field(i).Interface().(time.Time)
			result[jsonKey] = timeField.Format(time.RFC3339)
		} else {
			result[jsonKey] = v.Field(i).Interface()
		}
	}

	// Merge additionalData into result
	for key, value := range additionalData {
		result[key] = value
	}

	return result
}
