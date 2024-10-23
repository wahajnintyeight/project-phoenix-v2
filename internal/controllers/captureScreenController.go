package controllers

import (
	"net/http"
	"project-phoenix/v2/internal/cache"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/pkg/helper"
)

type CaptureScreenController struct {
}

func (cs *CaptureScreenController) CaptureScreen(w http.ResponseWriter, r *http.Request) (int, error) {

	data := "capture"
	dataInterface, e := helper.StringToInterface(data)
	if e != nil {
		return int(enum.CAPTURE_SCREEN_EVENT_FAILED), e
	}
	channelName := "capture-screen"
	isPub, e := cache.GetInstance().PublishMessage(dataInterface, channelName)
	if e != nil {
		return int(enum.CAPTURE_SCREEN_EVENT_FAILED), e
	}
	if isPub == true {
		return int(enum.CAPTURE_SCREEN_EVENT_SENT), nil
	}
	return -1, nil
}
