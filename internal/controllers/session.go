package controllers

import (
	"fmt"
	"net/http"
)

type SessionController struct {
}

func (sc *SessionController) CreateSession(w http.ResponseWriter, r *http.Request) {

	fmt.Println("Create Session")
}
