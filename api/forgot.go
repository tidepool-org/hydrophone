package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/mdblp/hydrophone/models"
	"github.com/mdblp/shoreline/schema"
	"github.com/tidepool-org/go-common/clients/status"
)

const (
	statusResetNotFound  = "No matching reset confirmation was found."
	statusResetAccepted  = "Password has been resetted."
	statusResetExpired   = "Password reset confirmation has expired."
	statusResetError     = "Error while resetting password; reset confirmation remains active until it expires."
	statusResetNoAccount = "No matching account for the email was found."
	statusResetPatient   = "Patient resetting his password."
)

type (
	//reset details reseting a users password
	resetBody struct {
		Key      string `json:"key"`
		Email    string `json:"email"`
		Password string `json:"password"`
		ShortKey string `json:"shortKey"`
	}
)

// @Summary Create a lost password request
// @Description  If the request is correctly formed, always returns a 200.
// @Description  If the email address is found in the system, this will:
// @Description     - Create a confirm record and a random key
// @Description     - Send an email with a link containing the key
// @Description  Visiting the URL in the email will fetch a page that offers the user the chance to accept or reject the lost password request.
// @Description  If accepted, the user must then create a new password that will replace the old one.
// @ID hydrophone-api-passwordReset
// @Accept  json
// @Produce  json
// @Param useremail path string true "user email"
// @Param x-tidepool-language header string false "User chosen language on 2 characters"
// @Param Accept-Language header string false "Browser defined languages as array of languages such as fr-FR"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "useremail was not provided"
// @Failure 422 {object} status.Status "Error when sending the email (probably caused by the mailling service"
// @Failure 500 {object} status.Status "Error finding the user, message returned:\"Error finding the user\" "
// @Router /send/forgot/{useremail} [post]
func (a *Api) passwordReset(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	var resetCnf *models.Confirmation
	var resetterLanguage string

	// on the "forgot password" page in Blip, the preferred language is now selected by listbox
	// even if not selected there is one by default so we should normally always end up with
	// a value in GetUserChosenLanguage()
	// however, just to play it safe, we can continue taking the browser preferred language
	// or ENglish as default values
	// In case the resetter is found a known user and has a language set, the language will be overridden in a later step
	if resetterLanguage = GetUserChosenLanguage(req); resetterLanguage == "" {
		if resetterLanguage = GetBrowserPreferredLanguage(req); resetterLanguage == "" {
			resetterLanguage = "en"
		}
	}

	email := vars["useremail"]
	if email == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	info, ok := req.URL.Query()["info"]

	if !ok || len(info[0]) < 1 {
		info = nil
	}

	// if the resetter is already registered we can use his preferences
	if resetUsr := a.findExistingUser(email, a.sl.TokenProvide()); resetUsr != nil {
		if !resetUsr.HasRole("patient") || a.Config.AllowPatientResetPassword {
			resetCnf, _ = models.NewConfirmation(models.TypePasswordReset, models.TemplateNamePasswordReset, "")
			info = nil
		} else {
			// patient
			log.Print(statusResetPatient)
			log.Printf("email used [%s]", email)
			if info != nil {
				resetCnf, _ = models.NewConfirmation(models.TypePatientPasswordInfo, models.TemplateNamePatientPasswordInfo, "")
			} else {
				resetCnf, _ = models.NewConfirmation(models.TypePatientPasswordReset, models.TemplateNamePatientPasswordReset, "")
			}
		}

		resetCnf.Email = email
		resetCnf.UserId = resetUsr.UserID

		// let's get the resetter user preferences
		resetterPreferences := &models.Preferences{}
		if err := a.seagull.GetCollection(resetCnf.UserId, "preferences", a.sl.TokenProvide(), resetterPreferences); err != nil {
			a.sendError(res, http.StatusInternalServerError,
				STATUS_ERR_FINDING_USR,
				"forgot password: error getting resetter user preferences: ",
				err.Error())
			return
		}
		// if resetter has a profile and a language we override the previously set language (browser's or "en")
		if resetterPreferences.DisplayLanguage != "" {
			resetterLanguage = resetterPreferences.DisplayLanguage
		}
	} else {
		log.Print(statusResetNoAccount)
		log.Printf("email used [%s]", email)
		resetCnf, _ = models.NewConfirmation(models.TypeNoAccount, models.TemplateNameNoAccount, "")
		resetCnf.Email = email
		//there is nothing more to do other than notify the user
		resetCnf.UpdateStatus(models.StatusCompleted)
	}

	if resetCnf != nil && (info != nil || a.addOrUpdateConfirmation(req.Context(), resetCnf, res)) {
		a.logAudit(req, "reset confirmation created")
		emailContent := map[string]interface{}{
			"Key":      resetCnf.Key,
			"Email":    resetCnf.Email,
			"ShortKey": resetCnf.ShortKey,
		}

		if a.createAndSendNotification(req, resetCnf, emailContent, resetterLanguage) {
			a.logAudit(req, "reset confirmation sent")
		} else {
			a.logAudit(req, "reset confirmation failed to be sent")
			log.Print("Something happened generating a passwordReset email")
			res.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
	}
	//unless no email was given we say its all good
	res.WriteHeader(http.StatusOK)
	return
}

//find the reset confirmation if it exists and hasn't expired
func (a *Api) findResetConfirmation(ctx context.Context, conf *models.Confirmation, res http.ResponseWriter) *models.Confirmation {

	log.Printf("findResetConfirmation: finding [%v]", conf)
	found, err := a.findExistingConfirmation(ctx, conf, res)
	if err != nil {
		log.Printf("findResetConfirmation: error [%s]\n", err.Error())
		a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
		return nil
	}
	if found == nil {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusNotFound, statusResetNotFound)}
		log.Printf("findResetConfirmation: not found [%s]\n", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
		return nil
	}
	if found.IsExpired() {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusUnauthorized, statusResetExpired)}
		log.Printf("findResetConfirmation: expired [%s]\n", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
		return nil
	}

	return found
}

// @Summary Accept the password change
// @Description  Likely to be invoked by the 'lost password' screen with the key that was included in the URL of the 'lost password' screen.
// @Description  For additional safety, the user will be required to manually enter the email address on the account as part of the UI,
// @Description  and also to enter a new password which will replace the password on the account.
// @Description  If this call is completed without error, the lost password request is marked as accepted.
// @Description  Otherwise, the lost password request remains active until it expires.
// @ID hydrophone-api-acceptPassword
// @Accept  json
// @Produce  json
// @Param payload body api.resetBody true "reset password details"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "Error while decoding the confirmation or while resetting password or missing key in the payload"
// @Failure 401 {object} status.Status "Password reset confirmation has expired"
// @Failure 404 {object} status.Status "No matching reset confirmation was found"
// @Failure 500 {object} status.Status "Internal error while searching the confirmation"
// @Router /confirm/accept/forgot [put]
func (a *Api) acceptPassword(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	defer req.Body.Close()
	var rb = &resetBody{}
	resetCnf := &models.Confirmation{}
	if err := json.NewDecoder(req.Body).Decode(rb); err != nil {
		log.Printf("acceptPassword: error decoding reset details %v\n", err)
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
		return
	}

	if rb.ShortKey != "" {
		// patient reset
		resetCnf = &models.Confirmation{Email: rb.Email, Type: models.TypePatientPasswordReset, ShortKey: rb.ShortKey, Status: models.StatusPending}
	} else {
		if rb.Key != "" {
			resetCnf = &models.Confirmation{Key: rb.Key, Email: rb.Email, Type: models.TypePasswordReset, Status: models.StatusPending}
		} else {
			log.Printf("acceptPassword: No key provided for %s\n", rb.Email)
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_FINDING_CONFIRMATION)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
			return
		}
	}

	if conf := a.findResetConfirmation(req.Context(), resetCnf, res); conf != nil {

		token := a.sl.TokenProvide()

		if usr := a.findExistingUser(rb.Email, token); usr != nil {

			if err := a.sl.UpdateUser(usr.UserID, schema.UserUpdate{Password: &rb.Password}, token); err != nil {
				log.Printf("acceptPassword: error updating password as part of password reset [%v]", err)
				status := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, statusResetError)}
				a.sendModelAsResWithStatus(res, status, http.StatusBadRequest)
				return
			}
			conf.UpdateStatus(models.StatusCompleted)
			if a.addOrUpdateConfirmation(req.Context(), conf, res) {
				a.logAudit(req, "password reset")
				a.sendModelAsResWithStatus(
					res,
					status.StatusError{Status: status.NewStatus(http.StatusOK, statusResetAccepted)},
					http.StatusOK,
				)
				return
			}
		}
	}
	return
}
