package middleware

import (
	"context"
	"log"
	"net/http"
	"os"
	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/enum"
	internal "project-phoenix/v2/internal/service-configs"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	// "project-phoenix/v2/internal/session"
)

type SessionMiddleware struct {
	redisClient       *redis.Client
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
	// val, err := sm.redisClient.Get(ctx, sessionID).Result()
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
	controller := controllers.GetControllerInstance(enum.SessionController, enum.MONGODB, "sessions")
	sessionController := controller.(*controllers.SessionController)
	val, err := sessionController.DoesSessionIDExist(sessionID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (sm *SessionMiddleware) Middleware(next http.Handler) http.Handler {
	log.Println("Session Middleware")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		godotenv.Load()
		token, err := sm.extractToken(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		newCtx := r.Context()
		session, err := sm.GetSession(newCtx, token)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
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
	log.Println("Session ID", sessionID)
	return sessionID, nil
}
