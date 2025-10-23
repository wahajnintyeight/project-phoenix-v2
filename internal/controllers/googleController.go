package controllers

import (
	"encoding/json"
	"net/http"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/google"
	"project-phoenix/v2/internal/model"
)

type GoogleController struct {
	CollectionName string
	DB             db.DBInterface
}

func (g *GoogleController) GetCollectionName() string {
	return ""
}

func (g *GoogleController) PerformIndexing() error {
	return nil
}

func (g *GoogleController) SearchYoutubeVideos(w http.ResponseWriter, r *http.Request) (int, interface{}, error) {
	searchRequestBody := model.GoogleSearchVideoRequestModel{}
	decodeErr := json.NewDecoder(r.Body).Decode(&searchRequestBody)
	if decodeErr != nil {
		return int(enum.ERROR), nil, decodeErr
	}
	res, e := google.SearchYoutube(searchRequestBody.Query, int64(searchRequestBody.MaxResults), searchRequestBody.NextPage, searchRequestBody.PrevPage)
	if e != nil {
		return int(enum.ERROR), nil, e
	}
	return int(enum.DATA_FETCHED), res, nil
}
