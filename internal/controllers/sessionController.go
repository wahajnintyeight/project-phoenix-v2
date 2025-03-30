package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"project-phoenix/v2/internal/cache"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/pkg/helper"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

type SessionController struct {
	CollectionName string
	DB             db.DBInterface
	SessionMutex   sync.Mutex
}

func (sc *SessionController) GetCollectionName() string {
	return "sessions"
}

func (sc *SessionController) PerformIndexing() error {
	// indexes := []interface{}{"sessionId", "createdAt"}
	indexes := []bson.D{
		{{Key: "sessionId", Value: 1}},
		{{Key: "createdAt", Value: 1}},
	}
	var validateErr error
	minutes := 120 //2 hours
	for _, index := range indexes {
		validateErr = sc.DB.ValidateIndexingTTL(sc.GetCollectionName(), index, minutes)
		if validateErr != nil {
			return validateErr
		}
	}
	return nil

}
func (sc *SessionController) CreateSession(w http.ResponseWriter, r *http.Request) (string, error) {
	// Add mutex lock to prevent concurrent session creation
	sc.SessionMutex.Lock()
	defer sc.SessionMutex.Unlock()
 
	filter := bson.M{
		"createdAt": bson.M{
			"$gte": time.Now().Add(-20 * time.Second),
		},
	}
	lastSession, err := sc.DB.FindOne(filter, sc.GetCollectionName())
	if err == nil && lastSession != nil {
		sessionData := map[string]interface{}{}
		e := helper.MapToStruct(lastSession, &sessionData)
		if e != nil {
			log.Println("Error while converting map to struct", e)
			return "", e
		}
		return sessionData["sessionID"].(string), nil
	}

	// Create new session if no recent one found
	sessionID, err := sc.generateSessionID(15)
	if err != nil {
		log.Println("Unable to generate session ID", err)
		return "", err
	}

	sessionData := map[string]interface{}{
		"sessionID": sessionID,
		"createdAt": time.Now(),
	}
	log.Println("Session Data", sessionData)

	_, err = sc.DB.Create(sessionData, sc.GetCollectionName())
	if err != nil {
		log.Println("Unable to store session in DB", err)
		return "", err
	}

	hours := 2
	sessionKey := "session:" + sessionID
	isAddedToRedis, err := cache.GetInstance().SetWithExpiry(sessionKey, map[string]interface{}{"sessionID": sessionData["sessionID"]}, hours)
	if err != nil {
		log.Println("Unable to store session in Redis", err)
		return "", err
	}

	if isAddedToRedis {
		w.Header().Set("sessionId", sessionID)
		return sessionID, nil
	}

	log.Println("Unable to store session in Redis")
	return "", nil

}

func (sc *SessionController) DoesSessionIDExist(sessionID string) (interface{}, error) {
	// sessionQuery := map[string]interface{}{
	// 	"sessionID": sessionID,
	// }
	sessionKey := "session:" + sessionID
	sessionData, err := cache.GetInstance().Get(sessionKey) //sc.DB.FindOne(sessionQuery, sc.GetCollectionName())
	// log.Println("Session Data", sessionData, err)
	if err != nil {
		log.Println("Error fetching session from DB", err)
		sessionData, err = sc.DB.FindOne(map[string]interface{}{"sessionID": sessionID}, sc.GetCollectionName())
		if err != nil {
			log.Println("Error fetching session from DB", err)
			return false, err
		} else {
			hours := 2
			_, err := cache.GetInstance().SetWithExpiry(sessionKey, map[string]interface{}{"sessionID": sessionID}, hours)
			if err != nil {
				log.Println("Error adding session to Redis", err)
				return false, err
			} else {
				return sessionData, nil
			}
		}
	} else {
		if sessionData != nil {
			return sessionData, nil
		} else {
			return false, nil
		}
	}
}

func (sc *SessionController) generateSessionID(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
