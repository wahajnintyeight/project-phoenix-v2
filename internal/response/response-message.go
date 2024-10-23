package response

import (
	"encoding/json"
	"net/http"
)

var responseMessage = map[int]string{
	-1:   "Internal Server Error",
	1000: "Users Fetched",
	1001: "Users Logged In",
	1002: "Error",
	1003: "Unable to Get User",
	1004: "Notifications Fetched",
	1005: "Notifications Failed To Fetch",
	1006: "Session Not Found",
	1007: "Session Success",
	1008: "Unable To Create Session",
	1009: "Data Fetched Successfully",
	1010: "Unable To Fetch Data",
	1011: "Data Inserted SuccessFully",
	1012: "Unable To Insert",
	1013: "Failed To Login",
	1014: "Invalid Password",
	1015: "Successfully Registered",
	1016: "This Email Already Exists",
	1017: "Welcome to Project Phoenix V2!",
	1018: "Message Generated Success!",
	1019: "Unable to fetch user",
	1020: "User logged out",
	1021: "Job Created",
	1022: "Job Not Created",
	1023: "Jobs Fetched",
	1024: "Jobs Not Fetched",
	1025: "Job Deleted",
	1026: "Job Not Deleted",
	1027: "Job does not exist",
	1028: "Job Expired",
	1029: "Job Not Expired",
	1030: "Job Already Expired",
	1031: "You are not logged in",
	1032: "Jobs Already Deleted",
	1033: "Jobs Deleted",
	1034: "Job Already Expired or Does Not Exists",
	1035: "Trip Not Created",
	1036: "Trip Created",
	1037: "Location Tracking Started",
	1038: "Location Tracking Not Started",
	1039: "OTP will be sent to your mobile number in a few seconds",
	1040: "OTP verified successfully",
	1041: "Trips Fetched",
	1042: "Unable to fetch Trips",
	1043: "Trip Not Deleted",
	1044: "Trip Deleted",
	1045: "User Location Fetched",
	1046: "User Location Not Fetched",
	1047: "Location Tracking Stopped",
	1048: "Location Tracking Not Stopped",
	1049: "Notifications Enabled",
	1050: "Notifications Disabled",
	1051: "Notifications Not Toggled",
	1052: "User Location History Fetched",
	1053: "User Location History Not Fetched",
	1054: "User Trip History Fetched",
	1055: "User Trip History Not Fetched",
	1056: "Total Distance Not Fetched",
	1057: "Total Distance Fetched",
	1058: "Geo Fence Added",
	1059: "Geo Fence Failed To Add",
	1060: "Geo Fence Fetched",
	1061: "Geo Fence Failed To Fetch",
	1062: "Geo Fence Deleted",
	1063: "Geo Fence Failed To Delete",
	1064: "Geo Fence with the same name already exists in this trip",
	1065: "Session Listed",
	1066: "Password Mismatch",
	1067: "Registration Failed",
	1068: "Session Header Not Found. Put 'sessionId' in the header.",
	1069: "Login Session Expired",
	1070: "Capture Screen Event Sent",
	1071: "Capture Screen Event Failed",
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

func SendErrorResponse(w http.ResponseWriter, responseCode int, response interface{}){
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(extractMessage(responseCode, response))
}

func extractMessage(responseCode int, response interface{}) interface{} {

	return MessageResponse{
		Code:    responseCode,
		Message: responseMessage[responseCode],
		Result:  response,
	}
}
