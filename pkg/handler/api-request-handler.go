package handler

import (
	"encoding/json"
	"fmt"
	"log"

	"net/http"
	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/controllers/middleware"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/response"
	internal "project-phoenix/v2/internal/service-configs"

	// "time"

	// "github.com/golang-jwt/jwt"
	"github.com/joho/godotenv"
	// "github.com/gorilla/mux"
)

type APIRequestHandler struct {
	Endpoint                string
	APIGatewayServiceConfig internal.ServiceConfig
}

func (apiHandler APIRequestHandler) GetEndpoint() string {
	apiRequestHandlerObj.Endpoint = apiHandler.Endpoint
	return apiRequestHandlerObj.Endpoint
}

func (apiHandler APIRequestHandler) SetEndpoint(endpoint string) {
	apiHandler.Endpoint = endpoint
}

var apiRequestHandlerObj APIRequestHandler

func (apiHandler APIRequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	requestMethod := r.Method
	apiHandler.GetEndpoint()
	log.Println("Serve HTTP | Method: ", requestMethod)
	switch requestMethod {
	case "PUT":
		apiHandler.PUTRoutes(w, r)
		break
	case "GET":
		GETRoutes(w, r)
		break
	case "POST":
		POSTRoutes(w, r)
	case "DELETE":
		apiHandler.DELETERoutes(w, r)
	}
}

func (apiHandler APIRequestHandler) DELETERoutes(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case apiRequestHandlerObj.Endpoint + "/deleteTrip":
		log.Println("Delete TRIP")
		controller := controllers.GetControllerInstance(enum.UserTripController, enum.MONGODB)
		userTripController := controller.(*controllers.UserTripController)
		code, message, er := userTripController.DeleteTrip(w, r)
		if er != nil {
			response.SendResponse(w, code, message)
			return
		} else {
			response.SendResponse(w, code, message)
			return
		}
	default:
		http.NotFound(w, r)
	}
}

func (apiHandler APIRequestHandler) PUTRoutes(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case apiRequestHandlerObj.Endpoint + "/createSession":
		controller := controllers.GetControllerInstance(enum.SessionController, enum.MONGODB)
		sessionController := controller.(*controllers.SessionController)
		res, ok := sessionController.CreateSession(w, r)
		if ok != nil {
			response.SendResponse(w, int(enum.SESSION_NOT_CREATED), res)
		} else {
			fmt.Println("Session created successfully")
			response.SendResponse(w, int(enum.SESSION_CREATED), res)
			return
		}
		break
	default:
		http.NotFound(w, r)
	}
}

func POSTRoutes(w http.ResponseWriter, r *http.Request) {
	//switch case for handling all the POST routes
	urlPath := r.URL.Path
	switch urlPath {
	case apiRequestHandlerObj.Endpoint + "/login":
		log.Println("Login")
		controller := controllers.GetControllerInstance(enum.UserController, enum.MONGODB)
		userController := controller.(*controllers.UserController)
		code, res, data, ok := userController.Login(w, r)
		if ok != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, data)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/register":
		log.Println("Register API Called")
		controller := controllers.GetControllerInstance(enum.UserController, enum.MONGODB)
		userController := controller.(*controllers.UserController)
		code, res, data, ok := userController.Register(w, r)
		if ok != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, data)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/logout":
		controller := controllers.GetControllerInstance(enum.UserController, enum.MONGODB)
		userController := controller.(*controllers.UserController)
		code, res, data, ok := userController.Logout(w, r)
		if ok != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, data)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/googleLogin":
		controller := controllers.GetControllerInstance(enum.UserController, enum.MONGODB)
		userController := controller.(*controllers.UserController)
		code, res, data, ok := userController.GoogleLogin(w, r)
		if ok != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, data)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/startTracking":
		controller := controllers.GetControllerInstance(enum.UserTripController, enum.MONGODB)
		userTripController := controller.(*controllers.UserTripController)
		code, res, data, ok := userTripController.StartTracking(w, r)
		if ok != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, data)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/stopTracking":
		controller := controllers.GetControllerInstance(enum.UserTripController, enum.MONGODB)
		userTripController := controller.(*controllers.UserTripController)
		code, res, data, ok := userTripController.StopTracking(w, r)
		if ok != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, data)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/createTrip":
		controller := controllers.GetControllerInstance(enum.UserTripController, enum.MONGODB)
		userTripController := controller.(*controllers.UserTripController)
		code, message, data, er := userTripController.CreateTrip(w, r)
		if er != nil {
			response.SendResponse(w, code, message)
			return
		} else {
			response.SendResponse(w, code, data)
			return

		}
	case apiRequestHandlerObj.Endpoint + "/deleteTrip":
		controller := controllers.GetControllerInstance(enum.UserTripController, enum.MONGODB)
		userTripController := controller.(*controllers.UserTripController)
		code, message, er := userTripController.DeleteTrip(w, r)
		if er != nil {
			response.SendResponse(w, code, message)
			return
		} else {
			response.SendResponse(w, code, message)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/handle-webhook":
		//log the request body
		request := map[string]interface{}{}
		decodeErr := json.NewDecoder(r.Body).Decode(&request)
		if decodeErr != nil {
			log.Println("Error decoding request body: ", decodeErr)
			response.SendErrorResponse(w, int(enum.ERROR), "Error decoding request body")
			return
		} else {
			log.Println("Request Body: ", request)
		}
		// response.SendErrorResponse(w, int(enum.ERROR), "Error decoding request body")

		response.SendResponse(w, int(enum.DATA_FETCHED), "Webhook Received")
		return
	case apiRequestHandlerObj.Endpoint + "/capture-screen":
		log.Println("Capture Screen")
		controller := controllers.GetControllerInstance(enum.CaptureScreenController, enum.MONGODB)
		captureScreenController := controller.(*controllers.CaptureScreenController)
		res, e := captureScreenController.CaptureScreen(w, r)
		if e != nil {
			response.SendResponse(w, int(enum.CAPTURE_SCREEN_EVENT_FAILED), e)
			return
		} else {
			response.SendResponse(w, int(enum.CAPTURE_SCREEN_EVENT_SENT), res)
			return
		}
	default:
		http.NotFound(w, r)
	}
}

func ValidateSession(w http.ResponseWriter, r *http.Request) bool {
	godotenv.Load()
	serviceConfigPath := "api-gateway"
	apiGatewayServiceConfig, err := internal.ReturnServiceConfig(serviceConfigPath)
	if err != nil {
		return false
	} else {
		exists := r.Context().Value(apiGatewayServiceConfig.(internal.ServiceConfig).SessionIDMiddlewareKey).(*middleware.Session)
		if exists == nil {
			http.Error(w, "Unauthorized - Session data not found", http.StatusUnauthorized)
			return false
		} else {
			return true
		}
	}
}

func GETRoutes(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	log.Print("GET Routes: ", urlPath)
	switch urlPath {
	case apiRequestHandlerObj.Endpoint + "/":
		log.Print("Welcome to API Gateway")
		response.SendResponse(w, int(enum.WELCOME), nil)
		break
	case apiRequestHandlerObj.Endpoint + "/getSessions":
		log.Println("Get All Sessions")
		response.SendResponse(w, int(enum.SESSIONS_LISTED), nil)
		break
	case apiRequestHandlerObj.Endpoint + "/getTrips":
		controller := controllers.GetControllerInstance(enum.UserTripController, enum.MONGODB)
		userTripController := controller.(*controllers.UserTripController)
		code, data, e := userTripController.ListAllTrips(w, r)
		if e != nil {
			response.SendResponse(w, code, e)
		}
		response.SendResponse(w, code, data)

		break
	case apiRequestHandlerObj.Endpoint + "/getCurrentLocation":
		controller := controllers.GetControllerInstance(enum.UserLocationController, enum.MONGODB)
		userLocationController := controller.(*controllers.UserLocationController)

		code, data, e := userLocationController.GetCurrentLocation(w, r)
		if e != nil {
			response.SendResponse(w, code, e)
		} else {
			response.SendResponse(w, code, data)
		}
		break
	case apiRequestHandlerObj.Endpoint + "/getLocationHistory":
		response.SendResponse(w, int(enum.DATA_FETCHED), map[string]interface{}{"code": 1022, "message": "Error", "result": nil})
		break
	default:
		http.NotFound(w, r)
	}
}
