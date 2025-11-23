package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"project-phoenix/v2/internal/broker"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/google"
	"project-phoenix/v2/internal/model"
	internal "project-phoenix/v2/internal/service-configs"
	"time"

	"github.com/google/uuid"
)

type GoogleController struct {
	CollectionName          string
	DB                      db.DBInterface
	APIGatewayServiceConfig internal.ServiceConfig
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

func (g *GoogleController) DownloadYoutubeVideos(w http.ResponseWriter, r *http.Request) (int, interface{}, error) {
	downloadRequestBody := model.GoogleDownloadVideoRequestModel{}
	decodeErr := json.NewDecoder(r.Body).Decode(&downloadRequestBody)
	if decodeErr != nil {
		return int(enum.ERROR), nil, decodeErr
	}
	// Generate unique download ID
	var downloadId string
	if downloadRequestBody.DownloadId == "" {
		downloadId = uuid.New().String()
	} else {
		downloadId = downloadRequestBody.DownloadId
	}

	// Create broker instance
	rabbitMQBroker := broker.CreateBroker(enum.RABBITMQ)

	// Prepare message for SSE service
	downloadMessage := map[string]interface{}{
		"downloadId": downloadId,
		"videoId":    downloadRequestBody.VideoId,
		"videoTitle": downloadRequestBody.VideoTitle,
		"format":     downloadRequestBody.Format,
		"bitRate":    downloadRequestBody.BitRate,
		"youtubeURL": downloadRequestBody.YoutubeURL,
		"quality": downloadRequestBody.Quality,
		"timestamp":  time.Now().UTC(),
		"status":     "queued",
	}

	// Publish to process-yt-video queue for SSE service to consume
	rabbitMQBroker.PublishMessage(downloadMessage, g.APIGatewayServiceConfig.ServiceName, "process-yt-video")

	// Return download ID immediately to client
	response := map[string]interface{}{
		"downloadId":  downloadId,
		"status":      "queued",
		"message":     "Download request queued successfully. Use SSE endpoint to track progress.",
		"sseEndpoint": fmt.Sprintf("/events/download-%s", downloadId),
	}

	return int(enum.DATA_FETCHED), response, nil
}
