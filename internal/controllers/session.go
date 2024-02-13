package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"project-phoenix/v2/internal/cache"
	"project-phoenix/v2/internal/db"
	"time"
)

type SessionController struct {
	CollectionName string
	DB             db.DBInterface
}

func (sc *SessionController) GetCollectionName() string {
	return "sessions"
}

func (sc *SessionController) CreateSession(w http.ResponseWriter, r *http.Request) (string, error) {

	sessionID, err := sc.generateSessionID(15)
	if err != nil {
		log.Println("Unable to generate session ID", err)
		return "", err
	} else {
		sessionData := map[string]interface{}{
			"sessionID": sessionID,
			"createdAt": time.Now(),
		}
		log.Println("Session Data", sessionData)
		_, err := sc.DB.Create(sessionData, sc.GetCollectionName())
		if err != nil {
			log.Println("Unable to store session in DB", err)
			return "", err
		} else {
			//help me out here
			isAddedToRedis, err := cache.GetInstance().Set(sessionID, map[string]interface{}{"sessionID": sessionData["sessionID"]})
			if err != nil {
				log.Println("Unable to store session in Redis", err)
				return "", err
			} else if isAddedToRedis == true {
				w.Header().Set("sessionId", sessionID)
				return sessionID, nil
			} else {
				log.Println("Unable to store session in Redis")
				return "", nil
			}
		}
	}
}

func (sc *SessionController) DoesSessionIDExist(sessionID string) (interface{}, error) {
	sessionQuery := map[string]interface{}{
		"sessionID": sessionID,
	}
	sessionData, err := sc.DB.FindOne(sessionQuery, sc.GetCollectionName())
	log.Println("Session Data", sessionData, err)
	if err != nil {
		log.Println("Error fetching session from DB", err)
		return false, err
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
