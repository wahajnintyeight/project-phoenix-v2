package controllers

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http/httptest"
	"project-phoenix/v2/internal/db"
	"strings"
	"testing"
)

func TestLLMAPIConfigIDFilter(t *testing.T) {
	id := primitive.NewObjectID()
	if got := llmAPIConfigIDFilter(id.Hex())["_id"]; got != id {
		t.Fatalf("lookup must use the stored ObjectID: got %v", got)
	}
	if got := llmAPIConfigIDFilter("legacy-config")["_id"]; got != "legacy-config" {
		t.Fatalf("legacy string IDs must remain strings: got %v", got)
	}
}

func TestLLMAPIConfigDocumentPreservesIDAndEncryptedKey(t *testing.T) {
	for _, id := range []interface{}{primitive.NewObjectID(), "legacy-config"} {
		config, err := llmAPIConfigFromDocument(bson.M{"_id": id, "encryptedApiKey": "encrypted-fixture", "isActive": true})
		if err != nil {
			t.Fatal(err)
		}
		if config.ID == "" || config.EncryptedAPIKey != "encrypted-fixture" || !config.IsActive {
			t.Fatal("database decoder lost configuration identity, encrypted credentials, or enabled state")
		}
		expectedID, ok := id.(string)
		if !ok {
			expectedID = id.(primitive.ObjectID).Hex()
		}
		if config.ToResponse().ID != expectedID {
			t.Fatal("list response did not preserve the exact configuration ID")
		}
	}
	if _, err := llmAPIConfigFromDocument(bson.M{"_id": ""}); err == nil {
		t.Fatal("empty ID was accepted")
	}
}

type llmConfigUpdateDB struct {
	db.DBInterface
	query  interface{}
	update interface{}
}

func (d *llmConfigUpdateDB) Update(query interface{}, update interface{}, collection string) (string, error) {
	d.query, d.update = query, update
	return "updated", nil
}

func TestLLMAPIConfigUpdateDoesNotNestSet(t *testing.T) {
	id := primitive.NewObjectID()
	database := &llmConfigUpdateDB{}
	controller := &LLMAPIConfigController{DB: database}
	request := httptest.NewRequest("PUT", "/llm-api-config?id="+id.Hex(), strings.NewReader(`{"isActive":false}`))
	_, _, err := controller.UpdateAPIConfig(httptest.NewRecorder(), request)
	if err != nil {
		t.Fatal(err)
	}
	if got := database.query.(bson.M)["_id"]; got != id {
		t.Fatalf("wrong lookup ID: %v", got)
	}
	fields := database.update.(bson.M)
	if _, nested := fields["$set"]; nested {
		t.Fatal("DB.Update already adds $set; configuration update must supply fields only")
	}
	if active, ok := fields["isActive"].(bool); !ok || active {
		t.Fatalf("deactivation was lost: %v", fields)
	}
}
