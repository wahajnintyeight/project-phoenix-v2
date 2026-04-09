package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"net/http"
	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/controllers/middleware"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/internal/response"
	internal "project-phoenix/v2/internal/service-configs"

	// "time"

	// "github.com/golang-jwt/jwt"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	case apiRequestHandlerObj.Endpoint + "/device":
		log.Println("Delete Device")
		controller := controllers.GetControllerInstance(enum.CaptureScreenController, enum.MONGODB)
		captureScreenController := controller.(*controllers.CaptureScreenController)
		code, err := captureScreenController.DeleteDevice(w, r)
		if err != nil {
			response.SendErrorResponse(w, code, err)
			return
		}
		response.SendResponse(w, code, nil)
		return
	case apiRequestHandlerObj.Endpoint + "/llm-api-config":
		log.Println("Delete LLM API Config")
		controller := controllers.GetControllerInstance(enum.LLMAPIConfigController, enum.MONGODB)
		configController := controller.(*controllers.LLMAPIConfigController)
		code, err := configController.DeleteAPIConfig(w, r)
		if err != nil {
			response.SendErrorResponse(w, code, err)
			return
		}
		response.SendResponse(w, code, nil)
		return
	default:
		// Check if it's a config/queries/{id} route
		if len(r.URL.Path) > len(apiRequestHandlerObj.Endpoint+"/config/queries/") &&
			r.URL.Path[:len(apiRequestHandlerObj.Endpoint+"/config/queries/")] == apiRequestHandlerObj.Endpoint+"/config/queries/" {
			log.Println("Delete Search Query")
			if !ValidateSession(w, r) {
				return
			}

			// Extract ID from path
			idStr := r.URL.Path[len(apiRequestHandlerObj.Endpoint+"/config/queries/"):]
			id, err := primitive.ObjectIDFromHex(idStr)
			if err != nil {
				response.SendErrorResponse(w, int(enum.ERROR), err)
				return
			}

			controller := controllers.GetControllerInstance(enum.ScraperConfigController, enum.MONGODB)
			configController := controller.(*controllers.ScraperConfigController)

			e := configController.DeleteQuery(id)
			if e != nil {
				response.SendErrorResponse(w, int(enum.DATA_NOT_FETCHED), e)
				return
			}

			response.SendResponse(w, int(enum.DATA_FETCHED), nil)
			return
		}
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
	case apiRequestHandlerObj.Endpoint + "/room/update":
		controller := controllers.GetControllerInstance(enum.ClipboardRoomController, enum.MONGODB)
		clipBoardController := controller.(*controllers.ClipboardRoomController)
		ok := clipBoardController.Update(w, r)
		if ok != nil {
			response.SendResponse(w, int(enum.ROOM_NOT_UPDATED), nil)
		} else {
			fmt.Println("Session created successfully")
			response.SendResponse(w, int(enum.ROOM_UPDATED), nil)
			return
		}
		break
	case apiRequestHandlerObj.Endpoint + "/llm-api-config":
		log.Println("Update LLM API Config")
		controller := controllers.GetControllerInstance(enum.LLMAPIConfigController, enum.MONGODB)
		configController := controller.(*controllers.LLMAPIConfigController)
		code, res, e := configController.UpdateAPIConfig(w, r)
		if e != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, res)
			return
		}
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
	case apiRequestHandlerObj.Endpoint + "/scan-devices":
		log.Println("Scan Devices")
		controller := controllers.GetControllerInstance(enum.CaptureScreenController, enum.MONGODB)
		captureScreenController := controller.(*controllers.CaptureScreenController)
		res, e := captureScreenController.ScanDevices(w, r)
		if e != nil {
			response.SendResponse(w, int(enum.SCAN_DEVICE_EVENT_FAILED), e)
			return
		} else {
			response.SendResponse(w, int(enum.SCAN_DEVICE_EVENT_SENT), res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/return-device-name":
		log.Println("Return Device Name")
		controller := controllers.GetControllerInstance(enum.CaptureScreenController, enum.MONGODB)
		captureScreenController := controller.(*controllers.CaptureScreenController)
		res, e := captureScreenController.ReturnDeviceName(w, r)
		if e != nil {
			response.SendResponse(w, int(enum.DEVICE_NAME_FAILED), e)
			return
		} else {
			response.SendResponse(w, int(enum.DEVICE_NAME_FETCHED), res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/ping":
		log.Println("Capture Screen")
		controller := controllers.GetControllerInstance(enum.CaptureScreenController, enum.MONGODB)
		captureScreenController := controller.(*controllers.CaptureScreenController)
		res, e := captureScreenController.PingDevice(w, r)
		if e != nil {
			response.SendResponse(w, int(enum.PING_DEVICE_EVENT_FAILED), e)
			return
		} else {
			response.SendResponse(w, int(enum.PING_DEVICE_EVENT_SENT), res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/room/create":
		log.Println("Create Room")
		controller := controllers.GetControllerInstance(enum.ClipboardRoomController, enum.MONGODB)
		clipboardRoomController := controller.(*controllers.ClipboardRoomController)
		code, roomData, e := clipboardRoomController.CreateRoom(w, r)
		if e != nil {
			response.SendResponse(w, code, e)
			return
		} else {
			response.SendResponse(w, code, roomData)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/room/join":
		log.Println("Join Room")
		controller := controllers.GetControllerInstance(enum.ClipboardRoomController, enum.MONGODB)
		clipboardRoomController := controller.(*controllers.ClipboardRoomController)
		_, roomData, e := clipboardRoomController.JoinRoom(w, r)
		if e != nil {
			response.SendResponse(w, int(enum.ROOM_NOT_FOUND), e)
			return
		} else {
			response.SendResponse(w, int(enum.ROOM_JOINED), roomData)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/search-yt-videos":
		log.Println("Search YT Videos")
		controller := controllers.GetControllerInstance(enum.GoogleController, enum.MONGODB)
		googleController := controller.(*controllers.GoogleController)
		_, res, e := googleController.SearchYoutubeVideos(w, r)
		if e != nil {
			response.SendResponse(w, int(enum.DATA_NOT_FETCHED), e)
			return
		} else {
			response.SendResponse(w, int(enum.DATA_FETCHED), res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/download-yt-videos":
		log.Println("Download YT Videos")
		controller := controllers.GetControllerInstance(enum.GoogleController, enum.MONGODB)
		googleController := controller.(*controllers.GoogleController)
		_, res, e := googleController.DownloadYoutubeVideos(w, r)
		if e != nil {
			response.SendResponse(w, int(enum.DATA_NOT_FETCHED), e)
			return
		} else {
			response.SendResponse(w, int(enum.DATA_FETCHED), res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/gollm/chat/completions":
		log.Println("GoLLM Chat Completion")
		controller := controllers.GetControllerInstance(enum.GoLLMController, enum.MONGODB)
		gollmController := controller.(*controllers.GoLLMController)
		code, res, e := gollmController.ChatCompletion(w, r)
		if e != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/gollm/completions":
		log.Println("GoLLM Text Completion")
		controller := controllers.GetControllerInstance(enum.GoLLMController, enum.MONGODB)
		gollmController := controller.(*controllers.GoLLMController)
		code, res, e := gollmController.TextCompletion(w, r)
		if e != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/gollm/ats/analyze-resume":
		log.Println("GoLLM ATS Analyze Resume")
		controller := controllers.GetControllerInstance(enum.GoLLMController, enum.MONGODB)
		gollmController := controller.(*controllers.GoLLMController)
		code, res, e := gollmController.ChatCompletionWithPrompt(w, r, model.ANALYZE_RESUME)
		if e != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/gollm/ats/enhance-description":
		log.Println("GoLLM ATS Enhance Description")
		controller := controllers.GetControllerInstance(enum.GoLLMController, enum.MONGODB)
		gollmController := controller.(*controllers.GoLLMController)
		code, res, e := gollmController.ChatCompletionWithPrompt(w, r, model.ENHANCE_DESCRIPTION)
		if e != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/gollm/ats/regenerate-item":
		log.Println("GoLLM ATS Regenerate Item")
		controller := controllers.GetControllerInstance(enum.GoLLMController, enum.MONGODB)
		gollmController := controller.(*controllers.GoLLMController)
		code, res, e := gollmController.ChatCompletionWithPrompt(w, r, model.REGENERATE_ITEM)
		if e != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/gollm/ats/regenerate-skills":
		log.Println("GoLLM ATS Regenerate Skills")
		controller := controllers.GetControllerInstance(enum.GoLLMController, enum.MONGODB)
		gollmController := controller.(*controllers.GoLLMController)
		code, res, e := gollmController.ChatCompletionWithPrompt(w, r, model.REGENERATE_SKILLS)
		if e != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/gollm/ats/scan":
		log.Println("GoLLM ATS Scan Resume")
		controller := controllers.GetControllerInstance(enum.GoLLMController, enum.MONGODB)
		gollmController := controller.(*controllers.GoLLMController)
		code, res, e := gollmController.ScanResumeATS(w, r)
		if e != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/gollm/test-connection":
		log.Println("GoLLM Test Connection")
		controller := controllers.GetControllerInstance(enum.GoLLMController, enum.MONGODB)
		gollmController := controller.(*controllers.GoLLMController)
		code, res, e := gollmController.TestConnection(w, r)
		if e != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/gollm/fetch-models":
		log.Println("GoLLM Fetch Models")
		controller := controllers.GetControllerInstance(enum.GoLLMController, enum.MONGODB)
		gollmController := controller.(*controllers.GoLLMController)
		code, res, e := gollmController.FetchOpenRouterModels(w, r)
		if e != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/llm-api-config":
		log.Println("Create LLM API Config")
		controller := controllers.GetControllerInstance(enum.LLMAPIConfigController, enum.MONGODB)
		configController := controller.(*controllers.LLMAPIConfigController)
		code, res, e := configController.CreateAPIConfig(w, r)
		if e != nil {
			response.SendResponse(w, code, res)
			return
		} else {
			response.SendResponse(w, code, res)
			return
		}
	case apiRequestHandlerObj.Endpoint + "/config/queries":
		log.Println("Create Search Query")
		if !ValidateSession(w, r) {
			return
		}
		controller := controllers.GetControllerInstance(enum.ScraperConfigController, enum.MONGODB)
		configController := controller.(*controllers.ScraperConfigController)

		var query model.SearchQuery
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
			response.SendErrorResponse(w, int(enum.ERROR), err)
			return
		}

		id, e := configController.CreateQuery(&query)
		if e != nil {
			response.SendErrorResponse(w, int(enum.INSERTION_FAILED), e)
			return
		}

		query.ID = id
		response.SendResponse(w, int(enum.INSERTION_SUCCESS), query)
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
	case apiRequestHandlerObj.Endpoint + "/devices":
		controller := controllers.GetControllerInstance(enum.CaptureScreenController, enum.MONGODB)
		screenCaptureController := controller.(*controllers.CaptureScreenController)

		code, d, e := screenCaptureController.ListDevices(1)
		if e != nil {
			response.SendErrorResponse(w, code, e)
		} else {
			response.SendResponse(w, code, d)
		}
		break
	case apiRequestHandlerObj.Endpoint + "/device":
		controller := controllers.GetControllerInstance(enum.CaptureScreenController, enum.MONGODB)
		log.Println("Get A Device")
		screenCaptureController := controller.(*controllers.CaptureScreenController)
		deviceId := r.URL.Query().Get("deviceId")

		if deviceId == "" {
			response.SendErrorResponse(w, int(enum.DEVICE_ID_NOT_SET), nil)
			break
		}
		code, d, e := screenCaptureController.ShowDeviceInfo(deviceId)
		if e != nil {
			response.SendErrorResponse(w, code, e)
		} else {
			response.SendResponse(w, code, d)
		}
		break
	case apiRequestHandlerObj.Endpoint + "/llm-api-configs":
		log.Println("List LLM API Configs")
		controller := controllers.GetControllerInstance(enum.LLMAPIConfigController, enum.MONGODB)
		configController := controller.(*controllers.LLMAPIConfigController)
		code, data, e := configController.ListAPIConfigs(w, r)
		if e != nil {
			response.SendResponse(w, code, e)
		} else {
			response.SendResponse(w, code, data)
		}
		break
	case apiRequestHandlerObj.Endpoint + "/llm-api-config":
		log.Println("Get LLM API Config")
		controller := controllers.GetControllerInstance(enum.LLMAPIConfigController, enum.MONGODB)
		configController := controller.(*controllers.LLMAPIConfigController)
		code, data, e := configController.GetAPIConfig(w, r)
		if e != nil {
			response.SendResponse(w, code, e)
		} else {
			response.SendResponse(w, code, data)
		}
		break
	case apiRequestHandlerObj.Endpoint + "/room/messages":
		controller := controllers.GetControllerInstance(enum.ClipboardRoomController, enum.MONGODB)
		clipboardRoomController := controller.(*controllers.ClipboardRoomController)
		code, data, e := clipboardRoomController.GetRoomMessages(w, r)
		if e != nil {
			response.SendResponse(w, code, e)
		} else {
			response.SendResponse(w, code, data)
		}
		break
	case apiRequestHandlerObj.Endpoint + "/keys":
		log.Println("List API Keys")
		if !ValidateSession(w, r) {
			return
		}
		controller := controllers.GetControllerInstance(enum.APIKeyController, enum.MONGODB)
		apiKeyController := controller.(*controllers.APIKeyController)

		// Parse query parameters
		page := 1
		if pageStr := r.URL.Query().Get("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}

		query := bson.M{}
		if provider := r.URL.Query().Get("provider"); provider != "" {
			query["provider"] = provider
		}
		if status := r.URL.Query().Get("status"); status != "" {
			query["status"] = status
		}

		totalPages, currentPage, keys, e := apiKeyController.FindAllWithPagination(query, page)
		if e != nil {
			response.SendErrorResponse(w, int(enum.DATA_NOT_FETCHED), e)
		} else {
			result := map[string]interface{}{
				"total_pages":  totalPages,
				"current_page": currentPage,
				"keys":         keys,
			}
			response.SendResponse(w, int(enum.DATA_FETCHED), result)
		}
		break
	case apiRequestHandlerObj.Endpoint + "/keys/valid":
		log.Println("List Valid API Keys")
		if !ValidateSession(w, r) {
			return
		}
		controller := controllers.GetControllerInstance(enum.APIKeyController, enum.MONGODB)
		apiKeyController := controller.(*controllers.APIKeyController)

		keys, e := apiKeyController.FindByStatus(model.StatusValid)
		if e != nil {
			response.SendErrorResponse(w, int(enum.DATA_NOT_FETCHED), e)
		} else {
			result := map[string]interface{}{
				"keys": keys,
			}
			response.SendResponse(w, int(enum.DATA_FETCHED), result)
		}
		break
	case apiRequestHandlerObj.Endpoint + "/stats":
		log.Println("Get API Key Statistics")
		if !ValidateSession(w, r) {
			return
		}
		controller := controllers.GetControllerInstance(enum.APIKeyController, enum.MONGODB)
		apiKeyController := controller.(*controllers.APIKeyController)

		stats, e := apiKeyController.GetStatistics()
		if e != nil {
			response.SendErrorResponse(w, int(enum.DATA_NOT_FETCHED), e)
		} else {
			response.SendResponse(w, int(enum.DATA_FETCHED), stats)
		}
		break
	case apiRequestHandlerObj.Endpoint + "/config/queries":
		log.Println("List Search Queries")
		if !ValidateSession(w, r) {
			return
		}
		controller := controllers.GetControllerInstance(enum.ScraperConfigController, enum.MONGODB)
		configController := controller.(*controllers.ScraperConfigController)

		queries, e := configController.GetAllQueries()
		if e != nil {
			response.SendErrorResponse(w, int(enum.DATA_NOT_FETCHED), e)
		} else {
			result := map[string]interface{}{
				"queries": queries,
			}
			response.SendResponse(w, int(enum.DATA_FETCHED), result)
		}
		break
	default:
		http.NotFound(w, r)
	}
}
