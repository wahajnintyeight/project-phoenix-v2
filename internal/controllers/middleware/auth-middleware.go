package middleware

import (
	"context"
	"log"
	"net/http"
	"project-phoenix/v2/internal/cache"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/internal/response"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/helper"

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
		// loginActivityQuery := map[string]interface{}{
		// "token": hash,
		// "email": email,
		// }
		// dbInstance, er := db.GetDBInstance(enum.MONGODB)
		// if er != nil {
		// log.Println("Error while getting DB Instance: ", er)
		// http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		// return
		// } else {
		sessionId := r.Header.Get("sessionId")
		loginActivityRedisKey := "login-activity:" + sessionId + ":" + email
		existingActivity, existingUserActivityError := cache.GetInstance().Get(loginActivityRedisKey)
		if existingUserActivityError != nil {
			log.Println("Auth Middleware | No User Activity in Redis")
			response.SendResponse(w, int(enum.LOGIN_SESSION_EXPIRED), nil)
			return
		}
		// existingActivity, existingUserActivityError := dbInstance.FindOne(loginActivityQuery, "loginactivities")
		log.Println("Existing Activity: ", existingActivity, "Error: ", existingUserActivityError)
		//store the user id in the request context
		loginActivity := &model.LoginActivity{}
		e := helper.InterfaceToStruct(existingActivity, &loginActivity)
		if e != nil {
			log.Println("Error while converting interface to struct: ", e)
			response.SendResponse(w, int(enum.LOGIN_SESSION_EXPIRED), nil)
			return
		}
		ctx := context.WithValue(r.Context(), "userId", loginActivity.UserID)
		r = r.WithContext(ctx)
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
		// }
		next.ServeHTTP(w, r)
	})
}
