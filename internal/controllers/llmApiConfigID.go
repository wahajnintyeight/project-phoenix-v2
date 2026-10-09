package controllers

import (
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"project-phoenix/v2/internal/model"
)

func llmAPIConfigFromDocument(document bson.M) (model.LLMAPIConfig, error) {
	var config model.LLMAPIConfig
	fields := make(bson.M, len(document))
	for key, value := range document {
		if key != "_id" {
			fields[key] = value
		}
	}
	data, err := bson.Marshal(fields)
	if err != nil {
		return config, err
	}
	if err := bson.Unmarshal(data, &config); err != nil {
		return config, err
	}
	switch id := document["_id"].(type) {
	case primitive.ObjectID:
		config.ID = id.Hex()
	case string:
		config.ID = id
	default:
		return config, fmt.Errorf("LLM configuration has no valid database ID")
	}
	if config.ID == "" {
		return config, fmt.Errorf("LLM configuration has an empty database ID")
	}
	return config, nil
}

func llmAPIConfigIDFilter(id string) bson.M {
	if objectID, err := primitive.ObjectIDFromHex(id); err == nil {
		return bson.M{"_id": objectID}
	}
	return bson.M{"_id": id}
}
