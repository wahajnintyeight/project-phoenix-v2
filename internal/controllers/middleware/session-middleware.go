package middleware

import (
	"context"
	"log"
	"net/http"
	"os"
	"project-phoenix/v2/internal/cache"
	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/response"
	internal "project-phoenix/v2/internal/service-configs"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	// "project-phoenix/v2/internal/session"
)

type SessionMiddleware struct {
	redisClient       cache.Redis
	mongoClient       *mongo.Client
	apiGatewayService internal.ServiceConfig
}

type Session struct {
	SessionID string    `bson:"sessionId"`
	CreatedAt time.Time `bson:"createdAt"`
	Expiry    time.Time `bson:"expiry"`
}

func NewSessionMiddleware() *SessionMiddleware {
	return &SessionMiddleware{}
}

func (sm *SessionMiddleware) GetSession(ctx context.Context, sessionID string) (interface{}, error) {
	// redisInstance := cache.GetInstance()
	// if redisInstance != nil {
	// 	redisSessionData, err := redisInstance.Get(sessionID)
	// 	if err != nil {
	// 		log.Println("Error fetching from Redis ", err)
	controller := controllers.GetControllerInstance(enum.SessionController, enum.MONGODB)
	sessionController := controller.(*controllers.SessionController)
	dbSessionData, sessionErr := sessionController.DoesSessionIDExist(sessionID)
	if sessionErr != nil {
		return nil, sessionErr
	} else {
		return dbSessionData, nil
	}
	// 	} else {
	// 		log.Println("Session Data from Redis", redisSessionData)
	// 		return redisSessionData, nil
	// 	}
	// } else {
	// 	log.Println("Redis client not initialized")
	// 	return nil, nil
	// }
	// if err == redis.Nil {
	// 	// If not found in Redis, look up in MongoDB
	// 	var sessionData bson.M
	// 	collection := sm.mongoClient.Database("yourDatabase").Collection("sessions")
	// 	if err := collection.FindOne(ctx, bson.M{"sessionID": sessionID}).Decode(&sessionData); err != nil {
	// 		return nil, err // Session not found in MongoDB
	// 	}

	// 	// Check session expiry
	// 	if sessionData["expiry"].(time.Time).Before(time.Now()) {
	// 		return nil, errors.New("session expired")
	// 	}

	// 	// Optionally, refresh session in Redis after fetching from MongoDB
	// 	return sessionData, nil
	// } else if err != nil {
	// 	return nil, err // Error fetching from Redis
	// }

}

func (sm *SessionMiddleware) Middleware(next http.Handler) http.Handler {
	log.Println("Session Middleware")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		godotenv.Load()
		token, err := sm.extractToken(r)
		if err != nil || token == "" {
			response.SendResponse(w, int(enum.SESSION_HEADER_NOT_FOUND), nil)
			return
		}

		newCtx := r.Context()
		session, err := sm.GetSession(newCtx, token)
		if err != nil {
			log.Println("Error fetching session", err)
			response.SendResponse(w, int(enum.SESSION_NOT_FOUND), nil)
			return
		}

		serviceConfigPath := os.Getenv("API_GATEWAY_SERVICE_CONFIG_PATH")
		apiGatewayServiceConfig, err := internal.ReturnServiceConfig(serviceConfigPath)
		ctx := context.WithValue(r.Context(), apiGatewayServiceConfig.(internal.ServiceConfig).SessionIDMiddlewareKey, session)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (sm *SessionMiddleware) extractToken(r *http.Request) (string, error) {
	sessionID := r.Header.Get("sessionId")
	log.Println("Extract Token - Session ID", sessionID)
	if sessionID == "" {
		log.Println("Session ID not found in header")
		return "", nil
	}
	log.Println("Session ID", sessionID)
	return sessionID, nil
}
