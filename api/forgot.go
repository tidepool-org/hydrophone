package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/hydrophone/models"
)

const (
	STATUS_RESET_NOT_FOUND  = "No matching reset confirmation was found."
	STATUS_RESET_ACCEPTED   = "Password has been reset."
	STATUS_RESET_EXPIRED    = "Password reset confirmation has expired."
	STATUS_RESET_ERROR      = "Error while resetting password; reset confirmation remains active until it expires."
	STATUS_RESET_NO_ACCOUNT = "No matching account for the email was found."
)

type (
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

	resetCnf, _ := models.NewConfirmation(models.TypePasswordReset, models.TemplateNamePasswordReset, "")
	resetCnf.Email = email

	if resetUsr := a.findExistingUser(resetCnf.Email, a.sl.TokenProvide()); resetUsr != nil {
		resetCnf.UserId = resetUsr.UserID
	} else {
		log.Print(STATUS_RESET_NO_ACCOUNT)
		log.Printf("email used [%s]", email)
		resetCnf, _ = models.NewConfirmation(models.TypeNoAccount, models.TemplateNameNoAccount, "")
		resetCnf.Email = email
		//there is nothing more to do other than notify the user
		resetCnf.UpdateStatus(models.StatusCompleted)
	}

	if a.addOrUpdateConfirmation(resetCnf, req.Context(), res) {
		a.logMetricAsServer("reset confirmation created")

		emailContent := map[string]interface{}{
			"Key":   resetCnf.Key,
			"Email": resetCnf.Email,
		}

		if a.createAndSendNotification(req, resetCnf, emailContent) {
			a.logMetricAsServer("reset confirmation sent")
		} else {
			a.logMetricAsServer("reset confirmation failed to be sent")
			log.Print("Something happened generating a passwordReset email")
		}
	}
	//unless no email was given we say its all good
	res.WriteHeader(http.StatusOK)
	return
}

//find the reset confirmation if it exists and hasn't expired
func (a *Api) findResetConfirmation(conf *models.Confirmation, res http.ResponseWriter) *models.Confirmation {

	log.Printf("findResetConfirmation: finding [%v]", conf)
	found, err := a.findExistingConfirmation(conf, res)
	if err != nil {
		log.Printf("findResetConfirmation: error [%s]\n", err.Error())
		a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
		return nil
	}
	if found == nil {
		statusErr := &status.StatusError{status.NewStatus(http.StatusNotFound, STATUS_RESET_NOT_FOUND)}
		log.Printf("findResetConfirmation: not found [%s]\n", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
		return nil
	}
	if found.IsExpired() {
		statusErr := &status.StatusError{status.NewStatus(http.StatusUnauthorized, STATUS_RESET_EXPIRED)}
		log.Printf("findResetConfirmation: expired [%s]\n", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
		return nil
	}

	return found
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
// status: 401 STATUS_RESET_EXPIRED the reset confirmaion has expired
// status: 400 STATUS_ERR_DECODING_CONFIRMATION issue decoding the accept body
// status: 400 STATUS_RESET_ERROR when we can't update the users password
// status: 404 STATUS_RESET_NOT_FOUND when no matching reset confirmation is found
func (a *Api) acceptPassword(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	defer req.Body.Close()
	var rb = &resetBody{}
	if err := json.NewDecoder(req.Body).Decode(rb); err != nil {
		log.Printf("acceptPassword: error decoding reset details %v\n", err)
		statusErr := &status.StatusError{status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
		return
	}

	resetCnf := &models.Confirmation{Key: rb.Key, Email: rb.Email, Type: models.TypePasswordReset}

	if conf := a.findResetConfirmation(resetCnf, res); conf != nil {

		token := a.sl.TokenProvide()

		if usr := a.findExistingUser(rb.Email, token); usr != nil {

			if err := a.sl.UpdateUser(usr.UserID, shoreline.UserUpdate{Password: &rb.Password}, token); err != nil {
				log.Printf("acceptPassword: error updating password as part of password reset [%v]", err)
				status := &status.StatusError{status.NewStatus(http.StatusBadRequest, STATUS_RESET_ERROR)}
				a.sendModelAsResWithStatus(res, status, http.StatusBadRequest)
				return
			}
			conf.UpdateStatus(models.StatusCompleted)
			if a.addOrUpdateConfirmation(conf, req.Context(), res) {
				//STATUS_RESET_ACCEPTED
				a.logMetricAsServer("password reset")
				a.sendModelAsResWithStatus(
					res,
					status.StatusError{status.NewStatus(http.StatusOK, STATUS_RESET_ACCEPTED)},
					http.StatusOK,
				)
				return
			}
		}
	}
	return
}
