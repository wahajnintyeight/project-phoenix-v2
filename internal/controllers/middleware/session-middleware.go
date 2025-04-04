package middleware

import (
	"context"
	"log"
	"net/http"
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

	controller := controllers.GetControllerInstance(enum.SessionController, enum.MONGODB)
	sessionController := controller.(*controllers.SessionController)
	dbSessionData, sessionErr := sessionController.DoesSessionIDExist(sessionID)
	if sessionErr != nil {
		return nil, sessionErr
	} else {
		return dbSessionData, nil
	}

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

		serviceConfigPath := "api-gateway"
		apiGatewayServiceConfig, err := internal.ReturnServiceConfig(serviceConfigPath)
		ctx := context.WithValue(r.Context(), apiGatewayServiceConfig.(internal.ServiceConfig).SessionIDMiddlewareKey, session)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (sm *SessionMiddleware) extractToken(r *http.Request) (string, error) {
	log.Println("All Headers", r.Header)
	sessionID := r.Header.Get("sessionId")
	log.Println("Extract Token - Session ID", sessionID)
	if sessionID == "" {
		log.Println("Session ID not found in header")
		return "", nil
	}
	log.Println("Session ID", sessionID)
	return sessionID, nil
}
