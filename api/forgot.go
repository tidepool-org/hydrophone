package api

import (
	"log"
	"net/http"
)

const (
	STATUS_NO_EMAIL_MATCH = ""
	STATUS_RESET_SENT     = ""
	STATUS_RESET_ACCEPTED = ""
)

//Send a password reset confirmation
//
// status: 200 STATUS_RESET_SENT
// status: 404 STATUS_NO_EMAIL_MATCH - no match is made on the email
func (a *Api) passwordReset(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if a.checkToken(res, req) {
		email := vars["useremail"]
		log.Printf("email for reset %s", email)
		res.WriteHeader(http.StatusNotImplemented)
		return
	}
	return
}

//Accept the password change
//
// status: 200 STATUS_RESET_ACCEPTED
// status: 404
func (a *Api) acceptPassword(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if a.checkToken(res, req) {
		res.WriteHeader(http.StatusNotImplemented)
		return
	}
	return
}
