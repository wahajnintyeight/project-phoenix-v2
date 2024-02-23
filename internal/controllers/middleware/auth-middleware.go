package middleware

import (
	"log"
	"net/http"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/response"
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
		log.Println("Auth Middleware | Request URL: ", r.URL.Path)
		email, hash, ok := r.BasicAuth()
		log.Println(email, hash)
		if !ok {
			log.Println("Auth Middleware | No Basic Auth")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		} else if email == "" || hash == "" {
			log.Println("Either Username or Hash not Provided in Basic Auth Header")
			response.SendResponse(w, int(enum.USER_NOT_FOUND), nil)
			return
		}
		loginActivityQuery := map[string]interface{}{
			"token": hash,
			"email": email,
		}
		dbInstance, er := db.GetDBInstance(enum.MONGODB)
		if er != nil {
			log.Println("Error while getting DB Instance: ", er)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		} else {
			existingActivity, existingUserActivityError := dbInstance.FindOne(loginActivityQuery, "loginactivities")
			log.Println("Existing Activity: ", existingActivity, "Error: ", existingUserActivityError)
			if existingUserActivityError == nil && existingActivity == nil {
				log.Println("Auth Middleware | No User Activity")
				response.SendResponse(w, int(enum.LOGIN_SESSION_EXPIRED), nil)
				return
			} else if existingUserActivityError != nil {
				//No login activity found. Throw error in response
				log.Println("Auth Middleware | User Login Activity Not Found")
				response.SendResponse(w, int(enum.LOGIN_SESSION_EXPIRED), nil)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
