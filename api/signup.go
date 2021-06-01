package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/mdblp/hydrophone/models"
	"github.com/mdblp/shoreline/schema"
	"github.com/tidepool-org/go-common/clients/status"
)

const (
	STATUS_SIGNUP_NOT_FOUND  = "No matching signup confirmation was found"
	STATUS_SIGNUP_NO_ID      = "Required userid is missing"
	STATUS_SIGNUP_NO_CONF    = "Required confirmation id is missing"
	STATUS_SIGNUP_ACCEPTED   = "User has had signup confirmed"
	STATUS_EXISTING_SIGNUP   = "User already has an existing valid signup confirmation"
	STATUS_SIGNUP_EXPIRED    = "The signup confirmation has expired"
	STATUS_SIGNUP_ERROR      = "Error while completing signup confirmation. The signup confirmation remains active until it expires"
	STATUS_ERR_FINDING_USR   = "Error finding user"
	STATUS_ERR_UPDATING_USR  = "Error updating user"
	STATUS_ERR_UPDATING_TEAM = "Error updating team"
	STATUS_NO_PASSWORD       = "User does not have a password"
	STATUS_MISSING_PASSWORD  = "Password is missing"
	STATUS_INVALID_PASSWORD  = "Password specified is invalid"
	STATUS_MISSING_BIRTHDAY  = "Birthday is missing"
	STATUS_INVALID_BIRTHDAY  = "Birthday specified is invalid"
	STATUS_MISMATCH_BIRTHDAY = "Birthday specified does not match patient birthday"
	STATUS_PATIENT_NOT_AUTH  = "Patient cannot be member of care team"
	STATUS_MEMBER_NOT_AUTH   = "Non patient users cannot be a patient of care team"
)

const (
	ERROR_NO_PASSWORD       = 1001
	ERROR_MISSING_PASSWORD  = 1002
	ERROR_INVALID_PASSWORD  = 1003
	ERROR_MISSING_BIRTHDAY  = 1004
	ERROR_INVALID_BIRTHDAY  = 1005
	ERROR_MISMATCH_BIRTHDAY = 1006
)

//try to find the signup confirmation
func (a *Api) findSignUp(ctx context.Context, conf *models.Confirmation, res http.ResponseWriter) *models.Confirmation {
	found, err := a.findExistingConfirmation(ctx, conf, res)
	if err != nil {
		log.Printf("findSignUp: error [%s]\n", err.Error())
		a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
		return nil
	}
	if found == nil {
		statusErr := &status.StatusError{status.NewStatus(http.StatusNotFound, STATUS_SIGNUP_NOT_FOUND)}
		log.Printf("findSignUp: not found [%s]\n", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
		return nil
	}

	return found
}

func (a *Api) updateSignupConfirmation(newStatus models.Status, res http.ResponseWriter, req *http.Request) {
	fromBody := &models.Confirmation{}
	if err := json.NewDecoder(req.Body).Decode(fromBody); err != nil {
		log.Printf("updateSignupConfirmation: error decoding signup to cancel [%s]", err.Error())
		statusErr := &status.StatusError{status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
		return
	}

	if fromBody.Key == "" {
		log.Printf("updateSignupConfirmation: %s", STATUS_SIGNUP_NO_CONF)
		statusErr := &status.StatusError{status.NewStatus(http.StatusBadRequest, STATUS_SIGNUP_NO_CONF)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
		return
	}

	if found, _ := a.findExistingConfirmation(req.Context(), fromBody, res); found != nil {

		updatedStatus := string(newStatus) + " signup"
		log.Printf("updateSignupConfirmation: %s", updatedStatus)
		found.UpdateStatus(newStatus)

		if a.addOrUpdateConfirmation(req.Context(), found, res) {
			a.logAudit(req, updatedStatus)
			res.WriteHeader(http.StatusOK)
			return
		}
	} else {
		log.Printf("updateSignupConfirmation: %s [%v]", STATUS_SIGNUP_NOT_FOUND, fromBody)
		a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusNotFound, STATUS_SIGNUP_NOT_FOUND), http.StatusNotFound)
		return
	}
}

// @Summary Patient account creation informative email
// @Description  Send an informative email to the patient to notify the account was successfully created
// @ID hydrophone-api-sendSignUpInformation
// @Accept  json
// @Produce  json
// @Param userid path string true "user id"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "userId was not provided, return:\"Required userid is missing\" "
// @Failure 403 {object} status.Status "Operation forbiden for this account, return detailed error"
// @Failure 422 {object} status.Status "Error when sending the email"
// @Failure 500 {object} status.Status "Error finding the user, message returned:\"Error finding the user\" "
// @Router /send/inform/{userid} [post]
func (a *Api) sendSignUpInformation(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	var signerLanguage string
	var newSignUp *models.Confirmation

	userID := vars["userid"]
	if userID == "" {
		log.Printf("sendSignUp %s", STATUS_SIGNUP_NO_ID)
		a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusBadRequest, STATUS_SIGNUP_NO_ID), http.StatusBadRequest)
		return
	}
	if usrDetails, err := a.sl.GetUser(userID, a.sl.TokenProvide()); err != nil {
		log.Printf("sendSignUp %s err[%s]", STATUS_ERR_FINDING_USER, err.Error())
		a.sendModelAsResWithStatus(res, status.StatusError{status.NewStatus(http.StatusInternalServerError, STATUS_ERR_FINDING_USER)}, http.StatusInternalServerError)
		return
	} else {
		if !usrDetails.HasRole("patient") {
			log.Printf("Clinician/Caregiver account [%s] cannot receive information message", usrDetails.UserID)
			a.sendModelAsResWithStatus(res, STATUS_ERR_CLINICAL_USR, http.StatusForbidden)
			return
		}
		if !usrDetails.EmailVerified {
			log.Printf("User [%s] is not yet verified", usrDetails.UserID)
			a.sendModelAsResWithStatus(res, STATUS_ERR_FINDING_VALIDATION, http.StatusForbidden)
			return
		} else {
			// send information message to patient
			var templateName = models.TemplateNamePatientInformation

			emailContent := map[string]interface{}{
				"Email": usrDetails.Emails[0],
			}

			newSignUp, _ = models.NewConfirmation(models.TypeInformation, templateName, usrDetails.UserID)
			newSignUp.Email = usrDetails.Emails[0]

			// this one may not work if the language is not set by the caller
			if signerLanguage = GetBrowserPreferredLanguage(req); signerLanguage == "" {
				signerLanguage = "en"
			}

			if a.createAndSendNotification(req, newSignUp, emailContent, signerLanguage) {
				log.Printf("signup information sent for %s", userID)
				a.logAudit(req, "signup information sent")
				res.WriteHeader(http.StatusOK)
				return
			} else {
				a.logAudit(req, "signup confirmation failed to be sent")
				log.Print("Something happened generating a signup email")
				res.WriteHeader(http.StatusUnprocessableEntity)
			}
		}
	}

}

// @Summary Send a signup confirmation email to a user
// @Description  This post is sent by the signup logic. In this state, the user account has been created but has a flag that
// @Description  forces the user to the confirmation-required page until the signup has been confirmed.
// @Description  It sends an email that contains a random confirmation link
// @Description  The email is sent in the language defined by "x-tidepool-language" header
// @Description  Otherwise if null "Accept-Language" header otherwise if null "ENglish"
// @ID hydrophone-api-sendSignUp
// @Accept  json
// @Produce  json
// @Param userid path string true "user id"
// @Param x-tidepool-language header string false "User chosen language on 2 characters"
// @Param Accept-Language header string false "Browser defined languages as array of languages such as fr-FR"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "userId was not provided, return:\"Required userid is missing\" "
// @Failure 401 {object} status.Status "Authorization token is missing or does not provided sufficient privileges"
// @Failure 403 {object} status.Status "Operation is forbiden, return detailed error"
// @Failure 500 {object} status.Status "Internal error while processing the confirmation, detailled error returned in the body"
// @Failure 422 {object} status.Status "Error when sending the email (probably caused by the mailling service)"
// @Router /send/signup/{userid} [post]
// @security TidepoolAuth
func (a *Api) sendSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	var signerLanguage string
	if token := a.token(res, req); token != nil {
		userId := vars["userid"]
		if userId == "" {
			log.Printf("sendSignUp %s", STATUS_SIGNUP_NO_ID)
			a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusBadRequest, STATUS_SIGNUP_NO_ID), http.StatusBadRequest)
			return
		}

		if !a.isAuthorizedUser(token, userId) {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		if usrDetails, err := a.sl.GetUser(userId, a.sl.TokenProvide()); err != nil {
			log.Printf("sendSignUp %s err[%s]", STATUS_ERR_FINDING_USER, err.Error())
			a.sendModelAsResWithStatus(res, status.StatusError{status.NewStatus(http.StatusInternalServerError, STATUS_ERR_FINDING_USER)}, http.StatusInternalServerError)
			return
		} else {

			// get any existing confirmations
			newSignUp, err := a.Store.FindConfirmation(req.Context(), &models.Confirmation{UserId: usrDetails.UserID, Type: models.TypeSignUp})
			if err != nil {
				log.Printf("sendSignUp: error [%s]\n", err.Error())
				a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
				return
			} else if newSignUp == nil {
				var templateName models.TemplateName
				var creatorID string

				if !usrDetails.HasRole("patient") {
					templateName = models.TemplateNameSignupClinic
				} else {
					templateName = models.TemplateNameSignup
				}

				newSignUp, _ = models.NewConfirmation(models.TypeSignUp, templateName, creatorID)
				newSignUp.UserId = usrDetails.UserID
				newSignUp.Email = usrDetails.Emails[0]
			} else if newSignUp.Email != usrDetails.Emails[0] {
				if err := a.Store.RemoveConfirmation(req.Context(), newSignUp); err != nil {
					log.Printf("sendSignUp: error deleting old [%s]", err.Error())
					a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
					return
				}

				if err := newSignUp.ResetKey(); err != nil {
					log.Printf("sendSignUp: error resetting key [%s]", err.Error())
					a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
					return
				}

				newSignUp.Email = usrDetails.Emails[0]
			} else {
				log.Printf("sendSignUp %s", STATUS_EXISTING_SIGNUP)
				a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusForbidden, STATUS_EXISTING_SIGNUP), http.StatusForbidden)
				return
			}

			if a.addOrUpdateConfirmation(req.Context(), newSignUp, res) {
				a.logAudit(req, "signup confirmation created")

				if err := a.addProfile(newSignUp); err != nil {
					log.Printf("sendSignUp: error when adding profile [%s]", err.Error())
					a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
					return
				} else {
					profile := &models.Profile{}
					if err := a.seagull.GetCollection(newSignUp.UserId, "profile", a.sl.TokenProvide(), profile); err != nil {
						a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, "sendSignUp: error getting user profile: ", err.Error())
						return
					}

					log.Printf("Sending email confirmation to %s with key %s", newSignUp.Email, newSignUp.Key)

					emailContent := map[string]interface{}{
						"Key":      newSignUp.Key,
						"Email":    newSignUp.Email,
						"FullName": profile.FullName,
					}

					if newSignUp.Creator.Profile != nil {
						emailContent["CreatorName"] = newSignUp.Creator.Profile.FullName
					}

					// on the "signup" page in Blip, the preferred language is now selected by listbox
					// even if not selected there is one by default so we should normally always end up with
					// a value in GetUserChosenLanguage()
					// however, just to play it safe, we can continue taking the browser preferred language
					// or ENglish as default values
					if signerLanguage = GetUserChosenLanguage(req); signerLanguage == "" {
						if signerLanguage = GetBrowserPreferredLanguage(req); signerLanguage == "" {
							signerLanguage = "en"
						}
					}

					if a.createAndSendNotification(req, newSignUp, emailContent, signerLanguage) {
						a.logAudit(req, "signup confirmation sent")
						res.WriteHeader(http.StatusOK)
						return
					} else {
						a.logAudit(req, "signup confirmation failed to be sent")
						log.Print("Something happened generating a signup email")
						res.WriteHeader(http.StatusUnprocessableEntity)
					}
				}
			}
		}
	}
}

// @Summary Resend a signup confirmation email to a user who have not confirmed yet
// @Description  If a user didn't receive the confirmation email and logs in, they're directed to the confirmation-required page which can
// @Description  offer to resend the confirmation email.
// @ID hydrophone-api-resendSignUp
// @Accept  json
// @Produce  json
// @Param useremail path string true "user email address"
// @Success 200 {string} string "OK"
// @Failure 404 {object} status.Status "Confirmation not found, return \"No matching signup confirmation was found\" "
// @Failure 422 {object} status.Status "Error when sending the email (probably caused by the mailling service)"
// @Failure 500 {object} status.Status "Internal error while regenerating the confirmation, detailled error returned in the body"
// @Router /resend/signup/{useremail} [post]
func (a *Api) resendSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	var signerLanguage string
	email := vars["useremail"]

	toFind := &models.Confirmation{Email: email, Status: models.StatusPending, Type: models.TypeSignUp}

	if found := a.findSignUp(req.Context(), toFind, res); found != nil {
		if err := a.Store.RemoveConfirmation(req.Context(), found); err != nil {
			log.Printf("resendSignUp: error deleting old [%s]", err.Error())
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
			return
		}

		if err := found.ResetKey(); err != nil {
			log.Printf("resendSignUp: error resetting key [%s]", err.Error())
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
			return
		}

		if a.addOrUpdateConfirmation(req.Context(), found, res) {
			a.logAudit(req, "signup confirmation recreated")

			if err := a.addProfile(found); err != nil {
				log.Printf("resendSignUp: error when adding profile [%s]", err.Error())
				a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
				return
			} else {
				profile := &models.Profile{}
				if err := a.seagull.GetCollection(found.UserId, "profile", a.sl.TokenProvide(), profile); err != nil {
					a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, "resendSignUp: error getting user profile: ", err.Error())
					return
				}

				log.Printf("Resending email confirmation to %s with key %s", found.Email, found.Key)

				emailContent := map[string]interface{}{
					"Key":      found.Key,
					"Email":    found.Email,
					"FullName": profile.FullName,
				}

				if found.Creator.Profile != nil {
					emailContent["CreatorName"] = found.Creator.Profile.FullName
				}

				// although technically there exists a profile at the signup stage, the preferred language would always be empty here
				// as it is set in the app and once the signup procedure is complete (after signup email has been confirmed)
				// so we rely on the language chosen by the user on the page through the list box
				// otherwise if null it will be browser preferences otherwise EN
				if signerLanguage = GetUserChosenLanguage(req); signerLanguage == "" {
					if signerLanguage = GetBrowserPreferredLanguage(req); signerLanguage == "" {
						signerLanguage = "en"
					}
				}

				if a.createAndSendNotification(req, found, emailContent, signerLanguage) {
					a.logAudit(req, "signup confirmation re-sent")
				} else {
					a.logAudit(req, "signup confirmation failed to be sent")
					log.Print("resendSignUp: Something happened trying to resend a signup email")
					res.WriteHeader(http.StatusUnprocessableEntity)
					return
				}
			}

			res.WriteHeader(http.StatusOK)
		}
	} else {
		a.sendError(res, http.StatusNotFound, STATUS_SIGNUP_NOT_FOUND, "resendSignUp: sign up not found")
	}
}

// @Summary Confirms the account creation
// @Description  This would be PUT by the web page at the link in the signup email. No authentication is required.
// @Description  When this call is made, the flag that prevents login on an account is removed, and the user is directed to the login page.
// @Description  If the user has an active cookie for signup (created with a short lifetime) we can accept the presence of that cookie to allow the actual login to be skipped.
// @ID hydrophone-api-acceptSignUp
// @Accept  json
// @Produce  json
// @Param confirmationid path string true "confirmation id"
// @Param details body models.Acceptance false "new password (optional)"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "confirmationid was not provided, return:\"Required confirmation id is missing\" "
// @Failure 404 {object} status.Status "Confirmation not found or expired"
// @Failure 409 {object} status.Status "Payload is missing or invalid. Return the detailled error"
// @Failure 422 {object} status.Status "Error when sending the email (probably caused by the mailling service)"
// @Failure 500 {object} status.Status "Error (internal) while updating the user account, return detailed error"
// @Router /accept/signup/{confirmationid} [put]
func (a *Api) acceptSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	confirmationId := vars["confirmationid"]

	if confirmationId == "" {
		log.Printf("acceptSignUp %s", STATUS_SIGNUP_NO_CONF)
		a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusBadRequest, STATUS_SIGNUP_NO_CONF), http.StatusBadRequest)
		return
	}

	toFind := &models.Confirmation{Key: confirmationId}

	if found := a.findSignUp(req.Context(), toFind, res); found != nil {
		if found.IsExpired() {
			a.sendError(res, http.StatusNotFound, STATUS_SIGNUP_EXPIRED, "acceptSignUp: expired")
			return
		}

		emailVerified := true
		updates := schema.UserUpdate{EmailVerified: &emailVerified}

		if _, err := a.sl.GetUser(found.UserId, a.sl.TokenProvide()); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, "acceptSignUp: error trying to get user to check email verified: ", err.Error())
			return
		}

		if err := a.sl.UpdateUser(found.UserId, updates, a.sl.TokenProvide()); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_UPDATING_USR, "acceptSignUp error trying to update user to be email verified: ", err.Error())
			return
		}

		found.UpdateStatus(models.StatusCompleted)
		if a.addOrUpdateConfirmation(req.Context(), found, res) {
			a.logAudit(req, "accept signup")
		}

		res.WriteHeader(http.StatusOK)
	}
}

// @Summary Dismiss an signup demand
// @Description  In the event that someone uses the wrong email address, the receiver could explicitly dismiss a signup attempt with this link (useful for identifying phishing attempts).
// @Description  This link would be some sort of parenthetical comment in the signup confirmation email, like "(I didn't try to sign up for YourLoops.)"
// @ID hydrophone-api-dismissSignUp
// @Accept  json
// @Produce  json
// @Param userid path string true "user id"
// @Param confirmation body models.Confirmation true "confirmation details"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "userid was not provided, or the payload is malformed (attributes missing or invalid). Return detailed error "
// @Failure 404 {object} status.Status "Cannot find a signup confirmation based on the provided key, return \"No matching signup confirmation was found\" "
// @Router /dismiss/signup/{userid} [put]
func (a *Api) dismissSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	userId := vars["userid"]

	if userId == "" {
		log.Printf("dismissSignUp %s", STATUS_SIGNUP_NO_ID)
		a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusBadRequest, STATUS_SIGNUP_NO_ID), http.StatusBadRequest)
		return
	}
	log.Print("dismissSignUp: dismissing for ", userId)
	a.updateSignupConfirmation(models.StatusDeclined, res, req)
	return
}

// @Summary Get Signup Confirmation requests
// @Description  Fetch pending confirmation requests for a given user.
// @ID hydrophone-api-getSignUp
// @Accept  json
// @Produce  json
// @Param userid path string true "user id"
// @Success 200 {array} models.Confirmation
// @Failure 400 {object} status.Status "userid was not provided"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provided sufficient privileges"
// @Failure 403 {object} status.Status "Operation is forbiden, return detailed error"
// @Failure 404 {object} status.Status "Cannot find a signup confirmation for this user, return \"No matching signup confirmation was found\" "
// @Router /signup/{userid} [get]
// @security TidepoolAuth
func (a *Api) getSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {

		userId := vars["userid"]

		if userId == "" {
			log.Printf("getSignUp %s", STATUS_SIGNUP_NO_ID)
			a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusBadRequest, STATUS_SIGNUP_NO_ID), http.StatusBadRequest)
			return
		}

		if !a.isAuthorizedUser(token, userId) {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		statuses := []models.Status{models.StatusPending}
		noTypes := []models.Type{}

		if signups, _ := a.Store.FindConfirmations(req.Context(), &models.Confirmation{UserId: userId, Type: models.TypeSignUp}, statuses, noTypes); signups == nil {
			log.Printf("getSignUp %s", STATUS_SIGNUP_NOT_FOUND)
			a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusNotFound, STATUS_SIGNUP_NOT_FOUND), http.StatusNotFound)
			return
		} else {
			a.logAudit(req, "get signups")
			log.Printf("getSignUp found %d for user %s", len(signups), userId)
			a.sendModelAsResWithStatus(res, signups, http.StatusOK)
			return
		}
	}
	return
}

// @Summary Cancel Signup request
// @Description Cancel a signup request for a given user
// @ID hydrophone-api-cancelSignUp
// @Accept  json
// @Produce  json
// @Param userid path string true "user id"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "userid was not provided, or the payload is malformed (attributes missing or invalid). Return detailed error "
// @Failure 404 {object} status.Status "Cannot find a signup confirmation based on the provided key, return \"No matching signup confirmation was found\" "
// @Router /signup/{userid} [put]
func (a *Api) cancelSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	userId := vars["userid"]

	if userId == "" {
		log.Printf("cancelSignUp: %s", STATUS_SIGNUP_NO_ID)
		a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusBadRequest, STATUS_SIGNUP_NO_ID), http.StatusBadRequest)
		return
	}
	log.Print("cancelSignUp: canceling for ", userId)
	a.updateSignupConfirmation(models.StatusCanceled, res, req)
	return
}

func IsValidPassword(password string) bool {
	ok, _ := regexp.MatchString(`\A\S{8,72}\z`, password)
	return ok
}

func IsValidDate(date string) bool {
	_, err := time.Parse("2006-01-02", date)
	return err == nil
}
