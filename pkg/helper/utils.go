package helper

import (
	"encoding/json"
	"log"
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
