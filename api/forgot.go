package api

import (
	"context"
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/tidepool-org/go-common/clients/shoreline"
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

// Create a lost password request
//
// If the request is correctly formed, always returns a 200, even if the email address was not found (this way it can't be used to validate email addresses).
//
// If the email address is found in the Tidepool system, this will:
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

	resetCnf, err := models.NewConfirmation(models.TypePasswordReset, models.TemplateNamePasswordReset, "")
	if err != nil {
		a.sendError(res, http.StatusInternalServerError, STATUS_ERR_CREATING_CONFIRMATION, err)
		return
	}

	resetCnf.Email = email

	if resetUsr := a.findExistingUser(resetCnf.Email, a.sl.TokenProvide()); resetUsr != nil {
		resetCnf.UserId = resetUsr.UserID
	} else {
		resetCnf, err = models.NewConfirmation(models.TypeNoAccount, models.TemplateNameNoAccount, "")
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_CREATING_CONFIRMATION, err)
			return
		}

		resetCnf.Email = email
		//there is nothing more to do other than notify the user
		resetCnf.UpdateStatus(models.StatusCompleted)
		a.logger.With(zap.String("email", email)).Info(STATUS_RESET_NO_ACCOUNT)
	}

	if a.addOrUpdateConfirmation(req.Context(), resetCnf, res) {
		a.logMetricAsServer("reset confirmation created")

		emailContent := map[string]interface{}{
			"Key":   resetCnf.Key,
			"Email": resetCnf.Email,
		}

		if a.createAndSendNotification(req, resetCnf, emailContent) {
			a.logMetricAsServer("reset confirmation sent")
		} else {
			a.logMetricAsServer("reset confirmation failed to be sent")
		}
	}
	//unless no email was given we say its all good
	res.WriteHeader(http.StatusOK)
}

// find the reset confirmation if it exists and hasn't expired
func (a *Api) findResetConfirmation(ctx context.Context, conf *models.Confirmation) (*models.Confirmation, bool, error) {
	a.logger.With("conf", conf).Debug("finding reset confirmation")
	found, err := a.Store.FindConfirmation(ctx, conf)
	if err != nil {
		return nil, false, err
	}
	if found == nil {
		return nil, false, nil
	}
	if found.IsExpired() {
		return nil, true, nil
	}

	return found, false, nil
}

// Accept the password change
//
// This call will be invoked by the lost password screen with the key that was included in the URL of the lost password screen.
// For additional safety, the user will be required to manually enter the email address on the account as part of the UI,
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
		a.sendError(res, http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION, err)
		return
	}

	resetCnf := &models.Confirmation{Key: rb.Key, Email: rb.Email, Status: models.StatusPending, Type: models.TypePasswordReset}

	conf, expired, err := a.findResetConfirmation(req.Context(), resetCnf)
	if err != nil {
		a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
		return
	}
	if expired {
		a.sendError(res, http.StatusNotFound, STATUS_RESET_EXPIRED)
		return
	}
	if conf == nil {
		a.sendError(res, http.StatusNotFound, STATUS_RESET_NOT_FOUND)
		return
	}

	if resetCnf.Key == "" || resetCnf.Email != conf.Email {
		a.sendError(res, http.StatusBadRequest, STATUS_RESET_ERROR)
		return
	}

	token := a.sl.TokenProvide()

	if usr := a.findExistingUser(rb.Email, token); usr != nil {

		if err := a.sl.UpdateUser(usr.UserID, shoreline.UserUpdate{Password: &rb.Password}, token); err != nil {
			a.sendError(res, http.StatusBadRequest, STATUS_RESET_ERROR, err, "updating user password")
			return
		}
		conf.UpdateStatus(models.StatusCompleted)
		if a.addOrUpdateConfirmation(req.Context(), conf, res) {
			a.logMetricAsServer("password reset")
			a.sendOK(res, STATUS_RESET_ACCEPTED)
			return
		}
	}
}
