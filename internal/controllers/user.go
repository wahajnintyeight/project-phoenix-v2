package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/pkg/helper"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
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

func (u *UserController) Login(w http.ResponseWriter, r *http.Request) (int, string, interface{}, error) {
	loginModel := model.LoginModel{}
	userModel := model.User{}
	decodeErr := json.NewDecoder(r.Body).Decode(&loginModel)
	if decodeErr == nil {
		userQuery := map[string]interface{}{
			"email": loginModel.Email,
		}
		existingUser, _ := u.DB.FindOne(userQuery, u.GetCollectionName())
		if existingUser == nil {
			return int(enum.USER_NOT_FOUND), "", nil, nil
		} else {
			e := helper.MapToStruct(existingUser, &userModel)
			if e != nil {
				log.Println("Error while converting map to struct", e)
				return int(enum.ERROR), "Error while converting map to struct", nil, e
			} else {
				if u.MatchPasswords(loginModel.Password, userModel.Password) {
					log.Println("User Model ", userModel)
					generatedToken, generateEr := GenerateJWT()
					if generateEr != nil {
						log.Println("Error while generating JWT", generateEr)
						return int(enum.ERROR), "Error while generating JWT", nil, generateEr
					} else {
						u.HandleLoginActivity(userModel, loginModel, r, generatedToken)

						//hide important info
						userModel.Password = ""
						userModel.ID = ""
						return int(enum.USER_LOGGED_IN), "", map[string]interface{}{"user": userModel, "token": generatedToken}, nil
					}

				} else {
					return int(enum.USER_LOGIN_FAILED), "", nil, nil
				}
			}
		}
	} else {
		log.Println("Unable to decode request body", decodeErr)
		return int(enum.ERROR), "Unable to decode request body", nil, decodeErr

	}
}

func (u *UserController) HandleLoginActivity(userModel model.User, loginModel model.LoginModel, r *http.Request, token string) {
	// Construct the query to check for existing loginActivity with the same sessionId and userId
	sessionId := r.Header.Get("sessionId")
	query := bson.M{
		"sessionId": sessionId,
		"userId":    userModel.ID,
	}

	// Prepare the update or insert data
	update := bson.M{
		"userId":      userModel.ID,
		"ip":          r.RemoteAddr,
		"createdAt":   time.Now(),
		"fcmKey":      loginModel.FcmKey,
		"isRider":     false,
		"isSpectator": false,
		"deviceName":  "",    // Add how you determine device name
		"token":       token, // Assuming you set the token after this function
	}

	// Call UpdateOrCreate with the constructed query and update data
	u.DB.UpdateOrCreate(query, update, "loginActivity")
}

func GenerateJWT() (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	godotenv.Load()
	appName := os.Getenv("APP_NAME")
	jwtKey := os.Getenv("JWT_KEY")
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		Issuer:    appName,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtKey))

	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (u *UserController) MatchPasswords(password string, hashedPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		log.Println("Error while comparing passwords", err)
		return false
	}
	return true
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
