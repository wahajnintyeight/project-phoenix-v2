package model

import (
	"encoding/json"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestLLMAPIConfigResponsePreservesIDWithoutCredentials(t *testing.T) {
	for _, id := range []primitive.ObjectID{primitive.NewObjectID(), primitive.NewObjectID()} {
		config := LLMAPIConfig{ID: id.Hex(), APIKey: "plain-fixture", EncryptedAPIKey: "encrypted-fixture"}
		data, err := json.Marshal(config.ToResponse())
		if err != nil {
			t.Fatal(err)
		}
		var response map[string]interface{}
		if err := json.Unmarshal(data, &response); err != nil {
			t.Fatal(err)
		}
		if got := response["id"]; got != id.Hex() {
			t.Fatalf("configuration lost its database ID: got %q, want %q", got, id.Hex())
		}
		for _, key := range []string{"apiKey", "encryptedApiKey"} {
			if _, present := response[key]; present {
				t.Fatalf("response leaked %s", key)
			}
		}
	}
}
