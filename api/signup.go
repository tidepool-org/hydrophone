package api

import (
	"log"
	"net/http"
	"time"

	"./../models"
	"github.com/tidepool-org/go-common/clients/status"
)

const (
	STATUS_SIGNUP_NOT_FOUND = "No matching signup confirmation was found"
	STATUS_SIGNUP_ACCEPTED  = "User has had signup confirmed"
	STATUS_SIGNUP_EXPIRED   = "The signup confirmation has expired"
	STATUS_SIGNUP_ERROR     = "Error while completing signup confirmation> The signup confirmation remains active until it expires"
)

type (
	//Content used to generate the signup email
	signUpEmailContent struct {
		Key   string
		Email string
	}
	//signup details
	signUpBody struct {
		Key   string `json:"key"`
		Email string `json:"email"`
	}
)

//find the reset confirmation if it exists and hasn't expired
func (a *Api) findSignUpConfirmation(conf *models.Confirmation, res http.ResponseWriter) (*models.Confirmation, error) {
	if signUpCnf := a.findExistingConfirmation(conf, res); signUpCnf != nil {

		expires := signUpCnf.Created.Add(time.Duration(a.Config.ResetTimeoutDays) * 24 * time.Hour)

		if time.Now().Before(expires) {
			return signUpCnf, nil
		}
		log.Printf("findSignUpConfirmation the confirmtaion has expired [%v]", signUpCnf)
		return nil, &status.StatusError{status.NewStatus(http.StatusUnauthorized, STATUS_RESET_EXPIRED)}
	}
	return nil, nil
}

//Send a signup confirmation email to a userid.
//
//This post is sent by the signup logic. In this state, the user account has been created but has a flag that
//forces the user to the confirmation-required page until the signup has been confirmed.
//It sends an email that contains a random confirmation link.
//
// status: 201
func (a *Api) sendSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	//NOTE: (We need some rules about how often you can attempt a signup with a given email address, to keep this from being used to spam people either deliberately or accidentally. This call should also be throttled at the system level to prevent distributed attacks.)

	res.WriteHeader(http.StatusNotImplemented)
	return
}

//If a user didn't receive the confirmation email and logs in, they're directed to the confirmation-required page which can offer to resend the confirmation email.
//
// status: 200
func (a *Api) resendSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	res.WriteHeader(http.StatusNotImplemented)
	return
}

//This would be PUT by the web page at the link in the signup email. No authentication is required.
//When this call is made, the flag that prevents login on an account is removed, and the user is directed to the login page.
//If the user has an active cookie for signup (created with a short lifetime) we can accept the presence of that cookie to allow the actual login to be skipped.
//
// status: 200
func (a *Api) acceptSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	res.WriteHeader(http.StatusNotImplemented)
	return
}

//In the event that someone uses the wrong email address, the receiver could explicitly dismiss a signup attempt with this link (useful for metrics and to identify phishing attempts).
//This link would be some sort of parenthetical comment in the signup confirmation email, like "(I didn't try to sign up for Tidepool.)"
//No authentication required.
//
// status: 200
func (a *Api) dismissSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	res.WriteHeader(http.StatusNotImplemented)
	return
}

// status: 200
func (a *Api) getSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	res.WriteHeader(http.StatusNotImplemented)
	return
}

// status: 200
func (a *Api) cancelSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	res.WriteHeader(http.StatusNotImplemented)
	return
}
