package response

import (
	"encoding/json"
	"net/http"
)

var responseMessage = map[int]string{
	1000: "Users Fetched",
}

type MessageResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Result  interface{} `json:"result"`
}

func SendResponse(w http.ResponseWriter, responseCode int, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(extractMessage(responseCode, response))
}

func extractMessage(responseCode int, response interface{}) interface{} {

	return MessageResponse{
		Code:    responseCode,
		Message: responseMessage[responseCode],
		Result:  response,
	}
}
