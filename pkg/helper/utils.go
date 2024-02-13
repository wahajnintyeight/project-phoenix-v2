package helper

import (
	"encoding/json"
	"log"

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
