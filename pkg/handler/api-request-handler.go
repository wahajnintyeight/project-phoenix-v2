package handler

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"os"
	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/controllers/middleware"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/response"
	internal "project-phoenix/v2/internal/service-configs"
	"time"

	"github.com/golang-jwt/jwt"
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
			log.Println("Request Body: ", request["steps"])
		}
		// response.SendErrorResponse(w, int(enum.ERROR), "Error decoding request body")

		response.SendResponse(w, int(enum.DATA_FETCHED), "Webhook Received")
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
	case apiRequestHandlerObj.Endpoint + "/signJWT":
		privateKeyFile, err := os.Open("pkg/handler/barclays_secret_key.txt")
		if err != nil {
			log.Fatalf("Error opening os private key: %v", err)
		}
		defer privateKeyFile.Close()

		// Read the private key file
		pemBytes, err := ioutil.ReadAll(privateKeyFile)
		if err != nil {
			log.Fatalf("Error reading private key file: %v", err)
		}

		// Decode PEM block
		block, _ := pem.Decode(pemBytes)
		if block == nil {
			log.Fatalf("Failed to decode PEM block containing private key")
		}

		// Parse the private key
		privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			log.Fatalf("Failed to parse private key: %v", err)
		}
		app_id := "bdn-ZEW0OPtCnFaTHERLcgBPO4c6qpA1DZgMEFn4oKBA"
		kid := "20332916849186968444295078258678577337"
		//I want to generate 3 random digit number and append to jti
		jti := "jti1111111111116734169315" + fmt.Sprintf("%d", (rand.Intn(90)+10))
		resourceType := r.URL.Query().Get("resourceType")
		log.Println("Resource Type:", resourceType)
		if resourceType != "consent" {
			resourceType = "token"
		}
		// Create the JWT token
		token := jwt.NewWithClaims(jwt.SigningMethodPS256, jwt.MapClaims{})
		switch resourceType {
		case "token":
			log.Println("Token")
			token = jwt.NewWithClaims(jwt.SigningMethodPS256, jwt.MapClaims{
				"iss": app_id,
				"sub": app_id,
				"exp": time.Now().Add(5 * time.Minute).Unix(),
				"iat": time.Now().Unix(),
				"jti": jti, //"jti111111111111673416931593",
				"aud": "https://token.sandbox.barclays.com/oauth/oauth20/token",
			})
			break
		case "consent":
			log.Println("Consent")
			consent := r.URL.Query().Get("consentId")
			log.Println("ConsentId:", consent)
			token = jwt.NewWithClaims(jwt.SigningMethodPS256, jwt.MapClaims{
				"iss":           app_id,
				"sub":           app_id,
				"response_type": "code id_token",
				"nonce":         "1234567890",
				"claims": map[string]interface{}{
					"userinfo": map[string]interface{}{
						"openbanking_intent_id": map[string]interface{}{
							"value":     consent,
							"essential": "true",
						},
					},
					"id_token": map[string]interface{}{
						"openbanking_intent_id": map[string]string{
							"value":     consent,
							"essential": "true",
						},
					},
					"acr": map[string]string{
						"essential": "true",
					},
				},
				"aud":        "https://token.sandbox.barclays.com/",
				"state":      "inprogressState",
				"scope":      "openid fundsconfirmations",
				"max_age":    86400,
				"acr_values": "urn:openbanking:psd2:sca urn:openbanking:psd2:ca",
			})
			break
		default:

		}
		// Here you might also need to set "kid" in the token's header
		// This is an example, replace "{KID}" with your actual key ID
		token.Header["kid"] = kid

		// Sign the token using the private key
		signedToken, err := token.SignedString(privateKey)
		if err != nil {
			log.Fatalf("Failed to sign token: %v", err)
		}

		response.SendResponse(w, int(enum.DATA_FETCHED), signedToken)
	case apiRequestHandlerObj.Endpoint + "/returnJWK":
		pemFile, err := os.Open("pkg/handler/public_key.pem")
		log.Println("PEM FILE", pemFile)
		if err != nil {
			fmt.Println(err)
			response.SendResponse(w, int(enum.ERROR), "Error in reading public key")
			return
		}
		defer pemFile.Close() // Don't forget to close the file

		pemData, err := ioutil.ReadAll(pemFile)
		if err != nil {
			fmt.Println(err)
			response.SendResponse(w, int(enum.ERROR), "Error in reading public key data")
			return
		}

		// Extract the PEM-encoded data block
		block, _ := pem.Decode(pemData)
		if block == nil {
			fmt.Println("no key found")
			response.SendResponse(w, int(enum.ERROR), "Error in decoding public key")
			return
		}

		// Parse the public key
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			fmt.Println(err)
			response.SendResponse(w, int(enum.ERROR), "Error in parsing public key")
			break
		}

		// Type assert to *rsa.PublicKey
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			fmt.Println("not an RSA public key")
			response.SendResponse(w, int(enum.ERROR), "Error in parsing RSA public key")
			break
		}

		// Construct the JWKS
		jwks := map[string]interface{}{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"use": "sig",
					"kid": "your-kid-here", // Replace with your actual kid
					"n":   base64.RawURLEncoding.EncodeToString(rsaPub.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaPub.E)).Bytes()),
				},
			},
		}

		// Convert JWKS to JSON
		jwksJson, err := json.MarshalIndent(jwks, "", "    ")
		if err != nil {
			fmt.Println(err)
			response.SendResponse(w, int(enum.ERROR), "Error in marshalling JWKS")
			return
		}

		// Set Content-Type header to application/json
		w.Header().Set("Content-Type", "application/json")

		// Write the JWKS JSON directly to the response
		w.WriteHeader(http.StatusOK) // Replace with the appropriate status code if needed
		_, err = w.Write(jwksJson)
		if err != nil {
			// Handle the error, maybe log it or send a different response
			fmt.Println(err)
			return
		}

		// Print the JWKS in JSON format
		fmt.Println(string(jwksJson))
		response.SendResponse(w, int(enum.DATA_FETCHED), (jwksJson))
	case apiRequestHandlerObj.Endpoint + "/getTrips":
		controller := controllers.GetControllerInstance(enum.UserTripController, enum.MONGODB)
		userTripController := controller.(*controllers.UserTripController)
		code, data, e := userTripController.ListAllTrips(w, r)
		if e != nil {
			response.SendResponse(w, code, e)
		}
		response.SendResponse(w, code, data)

		break
	default:
		http.NotFound(w, r)
	}
}
