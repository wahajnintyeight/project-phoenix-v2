package helper

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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

// Converts a map to string
func MapToString(data map[string]interface{}) string {
	dataByte, _ := MarshalBinary(data)
	return string(dataByte)
}

// Converts an interface{} to a string
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

/*
MapToStruct takes a map[string]interface{} and a target interface{} as input. It attempts to marshal the map into a JSON byte slice,
*/
func MapToStruct(data map[string]interface{}, target interface{}) error {
	dataByte, _ := MarshalBinary(data)
	err := UnmarshalBinary(dataByte, target)
	if err != nil {
		log.Println("Failed to unmarshal data: ", err)
		return err
	}
	return nil
}

/*
StructToMap takes an interface{} as input and attempts to marshal it into a map[string]interface{}.
*/
func StructToMap(data interface{}) (map[string]interface{}, error) {
	var result map[string]interface{}
	dataByte, err := MarshalBinary(data)
	if err != nil {
		log.Println("Failed to marshal data: ", err)
		return nil, err
	}

	err = UnmarshalBinary(dataByte, &result)
	if err != nil {
		log.Println("Failed to unmarshal data: ", err)
		return nil, err
	}
	return result, nil
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

func InterfaceToStruct(data interface{}, target interface{}) error {
	dataByte, err := MarshalBinary(data)
	if err != nil {
		log.Println("Failed to marshal data: ", err)
		return err
	}
	err = UnmarshalBinary(dataByte, &target)
	if err != nil {
		log.Println("Failed to unmarshal data: ", err)
		return err
	}
	return nil
}

func JSONStringToStruct(data interface{}, target interface{}) error {
	// Convert data to JSON string
	jsonData, ok := data.(string)
	if !ok {
		return fmt.Errorf("data is not a JSON string")
	}

	// Unmarshal JSON string into target struct
	err := json.Unmarshal([]byte(jsonData), target)
	if err != nil {
		log.Println("Failed to unmarshal JSON: ", err)
		return err
	}
	return nil
}

func StringToInterface(data string) (interface{}, error) {
	var result interface{}
	
	// Try to unmarshal the string as JSON
	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		// If it's not valid JSON, return the string as interface{}
		fmt.Println("Warning: Input is not valid JSON, returning as string in interface{}")
		return interface{}(data), nil
	}
	
	return result, nil
}

func BytesToString(data []byte) string {
	return string(data[:])
}

func GetCurrentTime() time.Time {
	return time.Now()
}

func GenerateTripID() string {
	// return 'trip-' +
	//add uuid4 to the trip id
	return "trip-" + uuid.New().String()
}

func GetCurrentUser(r *http.Request) string {
	return r.Context().Value("userId").(string)
}

func StringToInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Println("Failed to convert string to int: ", err)
		return 0
	}
	return i
}

func StringToObjectId(s string) primitive.ObjectID {
	oid, e := primitive.ObjectIDFromHex(s)
	if e != nil {
		log.Println("Failed to convert string to object id: ", e)
		return primitive.NilObjectID
	}
	return oid
}

func FloatToString(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
