package handler

import (
	"fmt"
	"log"
	"net/http"
	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/controllers/middleware"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/response"
	internal "project-phoenix/v2/internal/service-configs"

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
		return
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
	default:
		http.NotFound(w, r)
	}
}
