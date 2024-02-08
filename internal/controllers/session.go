package controllers

import (
	"fmt"
	"net/http"
)

type SessionController struct {
}

func (sc *SessionController) CreateSession(w http.ResponseWriter, r *http.Request) (string, error) {

	fmt.Println("Create Session")
	return "success", nil
}
