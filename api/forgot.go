package api

import (
	"encoding/json"
	"log"
	"net/http"

	"./../models"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
)

const (
	STATUS_NO_RESET_MATCH = "No matching reset confirmation was found"
	STATUS_RESET_ACCEPTED = "Password has been reset"
	STATUS_RESET_ERROR    = "Error while reseting password, reset confirmation remains active until it expires"
)

type (
	//Content used to generate the reset email
	resetEmailContent struct {
		Key   string
		Email string
	}
	//reset details reseting a users password
	resetBody struct {
		Key      string `json:"key"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
)

//Create a lost password request
//
//If the request is correctly formed, always returns a 200, even if the email address was not found (this way it can't be used to validate email addresses).
//
//If the email address is found in the Tidepool system, this will:
// - Create a confirm record and a random key
// - Send an email with a link containing the key
//
// Visiting the URL in the email will fetch a page that offers the user the chance to accept or reject the lost password request.
// If accepted, the user must then create a new password that will replace the old one.
//
// status: 200
// status: 400 no email given
func (a *Api) passwordReset(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	email := vars["useremail"]
	if email == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	resetCnf, _ := models.NewConfirmation(models.TypePasswordReset, "")
	resetCnf.Email = email

	//can we find the user?
	token := a.sl.TokenProvide()

	if resetUsr := a.findExistingUser(resetCnf.Email, token); resetUsr != nil {
		resetCnf.UserId = resetUsr.UserID
	}

	if a.addOrUpdateConfirmation(resetCnf, res) {
		a.logMetric("reset confirmation created", req)

		emailContent := &resetEmailContent{
			Key:   resetCnf.Key,
			Email: resetCnf.Email,
		}

		if a.createAndSendNotfication(resetCnf, emailContent) {
			a.logMetric("reset confirmation sent", req)
		}
	}
	//unless no email was given we say its all good
	res.WriteHeader(http.StatusOK)
	return
}

//Accept the password change
//
//This call will be invoked by the lost password screen with the key that was included in the URL of the lost password screen.
//For additional safety, the user will be required to manually enter the email address on the account as part of the UI,
// and also to enter a new password which will replace the password on the account.
//
// If this call is completed without error, the lost password request is marked as accepted.
// Otherwise, the lost password request remains active until it expires.
//
// status: 200 STATUS_RESET_ACCEPTED
// status: 400 STATUS_ERR_DECODING_CONFIRMATION issue decoding the accept body
// status: 400 STATUS_RESET_ERROR when we can't update the users password
// status: 404 STATUS_NO_RESET_MATCH when no matching reset confirmation is found
func (a *Api) acceptPassword(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if a.checkToken(res, req) {

		defer req.Body.Close()
		var rb = &resetBody{}
		if err := json.NewDecoder(req.Body).Decode(rb); err != nil {
			log.Printf("acceptPassword: error decoding reset details %v\n", err)
			statusErr := &status.StatusError{status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
			return
		}

		resetCnf := &models.Confirmation{Key: rb.Key, Email: rb.Email}

		if conf := a.findExistingConfirmation(resetCnf, res); conf != nil {

			if usr := a.findExistingUser(rb.Email, req.Header.Get(TP_SESSION_TOKEN)); usr != nil {

				if err := a.sl.UpdateUser(shoreline.UserUpdate{*usr, rb.Password}, req.Header.Get(TP_SESSION_TOKEN)); err != nil {
					log.Printf("Error updating password as part of password reset [%v]", err)
					status := &status.StatusError{status.NewStatus(http.StatusBadRequest, STATUS_RESET_ERROR)}
					a.sendModelAsResWithStatus(res, status, http.StatusBadRequest)
					return
				}
				conf.UpdateStatus(models.StatusCompleted)
				if a.addOrUpdateConfirmation(conf, res) {
					a.logMetric("password reset", req)
					status := &status.StatusError{status.NewStatus(http.StatusOK, STATUS_RESET_ACCEPTED)}
					a.sendModelAsResWithStatus(res, status, http.StatusOK)
					return
				}
			}
		}
		statusErr := &status.StatusError{status.NewStatus(http.StatusNotFound, STATUS_NO_RESET_MATCH)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
		return
	}
	return
}
