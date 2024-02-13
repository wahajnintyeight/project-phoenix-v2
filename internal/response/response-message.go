package response

import (
	"encoding/json"
	"net/http"
)

var responseMessage = map[int]string{
	1000: "Welcome to Project Phoenix V2",
	1006: "Session Not Found",
	1007: "Session Created",
	1008: "Session Not Created",
	1010: "Please provide a Session ID, use the 'sessionId' header",
	1022: "Password Mismatch",
	1016: "Registration Successful",
	1017: "User Already Exists",
	1019: "Registration Failed",
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
