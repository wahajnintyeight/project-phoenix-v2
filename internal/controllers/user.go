package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/pkg/helper"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type UserController struct {
	CollectionName string
	DB             db.DBInterface
}

func (u *UserController) GetCollectionName() string {
	return "users"
}

func (u *UserController) Register(w http.ResponseWriter, r *http.Request) (int, string, interface{}, error) {
	userModel := model.User{}
	registerModel := model.RegisterModel{}
	decodeErr := json.NewDecoder(r.Body).Decode(&registerModel)
	if decodeErr == nil {
		log.Println("Login Body", registerModel)
		log.Println("Decoded", userModel)
		if registerModel.Password != registerModel.ConfirmPassword {
			// response.SendResponse(w, http.StatusBadRequest, "Passwords do not match")
			return int(enum.PASSWORD_MISMATCH), "", nil, nil
		}
		//now let's hash the password, but first let's check if the user exists
		userQuery := map[string]interface{}{
			"email": registerModel.Email,
		}
		user, _ := u.DB.FindOne(userQuery, u.GetCollectionName())
		if user == nil {
			hashedPassword, hashErr := hashPassword(registerModel.Password)
			if hashErr != nil {
				log.Println("Error while hashing password", hashErr)
				return int(enum.ERROR), "Error while hashing password", nil, hashErr
			} else {
				//save user
				userModel = registerModel.User
				userModel.CreatedAt = time.Now()
				userModel.UpdatedAt = time.Now()
				userModel.Password = hashedPassword

				log.Println("User Model", userModel)
				insertedUser, userErr := u.DB.Create(userModel, u.GetCollectionName())
				if userErr != nil {
					log.Println("Error while creating user", userErr)
					return int(enum.REGISTER_FAILED), "", nil, userErr
				}
				log.Println("Inserted User", insertedUser)
				userModel.ID = helper.InterfaceToString(insertedUser["_id"])
				return int(enum.REGISTERED_SUCCESS), "", userModel, nil
			}
		} else {
			log.Println("User exists")
			// response.SendResponse(w, http.StatusBadRequest, "User already exists")
			return int(enum.EMAIL_EXISTS), "", nil, nil
		}

	} else {
		log.Println("Unable to decode request body", decodeErr)
		return -1, "Unable to decode request body", nil, decodeErr

	}
}

func hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		log.Println("Error while hashing password", err)
		return "", err
	}
	return string(hashedPassword), nil
}

func (u *UserController) DoesSessionIDExist(sessionID string) (interface{}, error) {
	sessionQuery := map[string]interface{}{
		"sessionID": sessionID,
	}
	sessionData, err := u.DB.FindOne(sessionQuery, u.GetCollectionName())
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

func (u *UserController) generateSessionID(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
