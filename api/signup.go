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

// status: 200
func (a *Api) sendSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	res.WriteHeader(http.StatusNotImplemented)
	return
}

// status: 200
func (a *Api) getSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	res.WriteHeader(http.StatusNotImplemented)
	return
}

// status: 200
func (a *Api) resendSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	res.WriteHeader(http.StatusNotImplemented)
	return
}

// status: 200
func (a *Api) acceptSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	res.WriteHeader(http.StatusNotImplemented)
	return
}

// status: 200
func (a *Api) dismissSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	res.WriteHeader(http.StatusNotImplemented)
	return
}

// status: 200
func (a *Api) cancelSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	res.WriteHeader(http.StatusNotImplemented)
	return
}
