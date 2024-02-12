package middleware

import (
	"log"
	"net/http"
	internal "project-phoenix/v2/internal/service-configs"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuthMiddleware struct {
	redisClient       *redis.Client
	mongoClient       *mongo.Client
	apiGatewayService internal.ServiceConfig
}

type UserData struct {
	UserName string `bson:"userName"`
	UserHash string `bson:"userHash"`
}

func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{}
}

func (a *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	log.Println("Auth Middleware")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		next.ServeHTTP(w, r)
	})
}
