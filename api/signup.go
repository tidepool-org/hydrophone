package api

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"time"

	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/hydrophone/models"
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
	STATUS_NO_PASSWORD       = "User does not have a password"
	STATUS_MISSING_PASSWORD  = "Password is missing"
	STATUS_INVALID_PASSWORD  = "Password specified is invalid"
	STATUS_MISSING_BIRTHDAY  = "Birthday is missing"
	STATUS_INVALID_BIRTHDAY  = "Birthday specified is invalid"
	STATUS_MISMATCH_BIRTHDAY = "Birthday specified does not match patient birthday"
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
func (a *Api) findSignUp(conf *models.Confirmation, res http.ResponseWriter) *models.Confirmation {
	found, err := a.findExistingConfirmation(conf, res)
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

//update an existing signup confirmation
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

	if found, _ := a.findExistingConfirmation(fromBody, res); found != nil {

		updatedStatus := string(newStatus) + " signup"
		log.Printf("updateSignupConfirmation: %s", updatedStatus)
		found.UpdateStatus(newStatus)

		if a.addOrUpdateConfirmation(found, res) {
			a.logMetricAsServer(updatedStatus)
			res.WriteHeader(http.StatusOK)
			return
		}
	} else {
		log.Printf("updateSignupConfirmation: %s [%v]", STATUS_SIGNUP_NOT_FOUND, fromBody)
		a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusNotFound, STATUS_SIGNUP_NOT_FOUND), http.StatusNotFound)
		return
	}
}

//Send a signup confirmation email to a userid.
//
//This post is sent by the signup logic. In this state, the user account has been created but has a flag that
//forces the user to the confirmation-required page until the signup has been confirmed.
//It sends an email that contains a random confirmation link.
//
// status: 201
// status: 400 STATUS_SIGNUP_NO_ID
// status: 401 STATUS_NO_TOKEN
// status: 403 STATUS_EXISTING_SIGNUP
// status: 500 STATUS_ERR_FINDING_USER
func (a *Api) sendSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	var signerLanguage string
	if token := a.token(res, req); token != nil {
		userId := vars["userid"]
		if userId == "" {
			log.Printf("sendSignUp %s", STATUS_SIGNUP_NO_ID)
			a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusBadRequest, STATUS_SIGNUP_NO_ID), http.StatusBadRequest)
			return
		}

		if permissions, err := a.tokenUserHasRequestedPermissions(token, userId, commonClients.Permissions{"root": commonClients.Allowed, "custodian": commonClients.Allowed}); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, err)
			return
		} else if permissions["root"] == nil && permissions["custodian"] == nil {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		if usrDetails, err := a.sl.GetUser(userId, a.sl.TokenProvide()); err != nil {
			log.Printf("sendSignUp %s err[%s]", STATUS_ERR_FINDING_USER, err.Error())
			a.sendModelAsResWithStatus(res, status.StatusError{status.NewStatus(http.StatusInternalServerError, STATUS_ERR_FINDING_USER)}, http.StatusInternalServerError)
			return
		} else {

			// get any existing confirmations
			newSignUp, err := a.Store.FindConfirmation(&models.Confirmation{UserId: usrDetails.UserID, Type: models.TypeSignUp})
			if err != nil {
				log.Printf("sendSignUp: error [%s]\n", err.Error())
				a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
				return
			} else if newSignUp == nil {
				var templateName models.TemplateName
				var creatorID string

				if usrDetails.IsClinic() {
					templateName = models.TemplateNameSignupClinic
				} else if usrDetails.IsCustodial() {
					if token.IsServer {
						templateName = models.TemplateNameSignupCustodial
					} else {
						tokenUserDetails, err := a.sl.GetUser(token.UserID, a.sl.TokenProvide())
						if err != nil {
							log.Printf("sendSignUp: error when getting token user [%s]", err.Error())
							a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
							return
						}

						creatorID = token.UserID

						if tokenUserDetails.IsClinic() {
							templateName = models.TemplateNameSignupCustodialClinic
						} else {
							templateName = models.TemplateNameSignupCustodial
						}
					}
				} else {
					templateName = models.TemplateNameSignup
				}

				newSignUp, _ = models.NewConfirmation(models.TypeSignUp, templateName, creatorID)
				newSignUp.UserId = usrDetails.UserID
				newSignUp.Email = usrDetails.Emails[0]
			} else if newSignUp.Email != usrDetails.Emails[0] {
				if err := a.Store.RemoveConfirmation(newSignUp); err != nil {
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

			if a.addOrUpdateConfirmation(newSignUp, res) {
				a.logMetric("signup confirmation created", req)

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

					// although technically there exists a profile at the signup stage, the preferred language would always be empty here
					// as it is set in the app and once the signup procedure is complete (after signup email has been confirmed)
					// -> get browser's or "en" for English in case there is no browser's
					if signerLanguage = getBrowserPreferredLanguage(req); signerLanguage == "" {
						signerLanguage = "en"
					}

					if a.createAndSendNotification(newSignUp, emailContent, signerLanguage) {
						a.logMetricAsServer("signup confirmation sent")
						res.WriteHeader(http.StatusOK)
						return
					} else {
						a.logMetric("signup confirmation failed to be sent", req)
						log.Print("Something happened generating a signup email")
					}
				}
			}
		}
	}
}

//If a user didn't receive the confirmation email and logs in, they're directed to the confirmation-required page which can
//offer to resend the confirmation email.
//
// status: 200
// status: 404 STATUS_SIGNUP_EXPIRED
func (a *Api) resendSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	var signerLanguage string
	email := vars["useremail"]

	toFind := &models.Confirmation{Email: email, Status: models.StatusPending}

	if found := a.findSignUp(toFind, res); found != nil {
		if err := a.Store.RemoveConfirmation(found); err != nil {
			log.Printf("resendSignUp: error deleting old [%s]", err.Error())
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
			return
		}

		if err := found.ResetKey(); err != nil {
			log.Printf("resendSignUp: error resetting key [%s]", err.Error())
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
			return
		}

		if a.addOrUpdateConfirmation(found, res) {
			a.logMetricAsServer("signup confirmation recreated")

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
				// -> get browser's or "en" for English in case there is no browser's
				if signerLanguage = getBrowserPreferredLanguage(req); signerLanguage == "" {
					signerLanguage = "en"
				}

				if a.createAndSendNotification(found, emailContent, signerLanguage) {
					a.logMetricAsServer("signup confirmation re-sent")
				} else {
					a.logMetricAsServer("signup confirmation failed to be sent")
					log.Print("resendSignUp: Something happened trying to resend a signup email")
				}
			}

			res.WriteHeader(http.StatusOK)
		}
	}
}

//This would be PUT by the web page at the link in the signup email. No authentication is required.
//When this call is made, the flag that prevents login on an account is removed, and the user is directed to the login page.
//If the user has an active cookie for signup (created with a short lifetime) we can accept the presence of that cookie to allow the actual login to be skipped.
//
// status: 200
// status: 400 STATUS_SIGNUP_NO_CONF
// status: 400 STATUS_NO_PASSWORD
// status: 400 STATUS_MISSING_PASSWORD
// status: 400 STATUS_INVALID_PASSWORD
// status: 400 STATUS_MISSING_BIRTHDAY
// status: 400 STATUS_INVALID_BIRTHDAY
// status: 400 STATUS_MISMATCH_BIRTHDAY
func (a *Api) acceptSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	confirmationId := vars["confirmationid"]

	if confirmationId == "" {
		log.Printf("acceptSignUp %s", STATUS_SIGNUP_NO_CONF)
		a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusBadRequest, STATUS_SIGNUP_NO_CONF), http.StatusBadRequest)
		return
	}

	toFind := &models.Confirmation{Key: confirmationId}

	if found := a.findSignUp(toFind, res); found != nil {
		if found.IsExpired() {
			a.sendError(res, http.StatusNotFound, STATUS_SIGNUP_EXPIRED, "acceptSignUp: expired")
			return
		}

		emailVerified := true
		updates := shoreline.UserUpdate{EmailVerified: &emailVerified}

		if user, err := a.sl.GetUser(found.UserId, a.sl.TokenProvide()); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, "acceptSignUp: error trying to get user to check email verified: ", err.Error())
			return

		} else if !user.PasswordExists {
			acceptance := &models.Acceptance{}
			if req.Body != nil {
				if err := json.NewDecoder(req.Body).Decode(acceptance); err != nil {
					a.sendErrorWithCode(res, http.StatusConflict, ERROR_NO_PASSWORD, STATUS_NO_PASSWORD, "acceptSignUp: error decoding acceptance: ", err.Error())
					return
				}
			}

			if acceptance.Password == "" {
				a.sendErrorWithCode(res, http.StatusConflict, ERROR_MISSING_PASSWORD, STATUS_MISSING_PASSWORD, "acceptSignUp: missing password")
				return
			}
			if !IsValidPassword(acceptance.Password) {
				a.sendErrorWithCode(res, http.StatusConflict, ERROR_INVALID_PASSWORD, STATUS_INVALID_PASSWORD, "acceptSignUp: invalid password specified")
				return
			}
			if acceptance.Birthday == "" {
				a.sendErrorWithCode(res, http.StatusConflict, ERROR_MISSING_BIRTHDAY, STATUS_MISSING_BIRTHDAY, "acceptSignUp: missing birthday")
				return
			}
			if !IsValidDate(acceptance.Birthday) {
				a.sendErrorWithCode(res, http.StatusConflict, ERROR_INVALID_BIRTHDAY, STATUS_INVALID_BIRTHDAY, "acceptSignUp: invalid birthday specified")
				return
			}

			profile := &models.Profile{}
			if err := a.seagull.GetCollection(found.UserId, "profile", a.sl.TokenProvide(), profile); err != nil {
				a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, "acceptSignUp: error getting the users profile: ", err.Error())
				return
			}

			if acceptance.Birthday != profile.Patient.Birthday {
				a.sendErrorWithCode(res, http.StatusConflict, ERROR_MISMATCH_BIRTHDAY, STATUS_MISMATCH_BIRTHDAY, "acceptSignUp: acceptance birthday does not match user patient birthday")
				return
			}

			updates.Password = &acceptance.Password
		}

		if err := a.sl.UpdateUser(found.UserId, updates, a.sl.TokenProvide()); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_UPDATING_USR, "acceptSignUp error trying to update user to be email verified: ", err.Error())
			return
		}

		found.UpdateStatus(models.StatusCompleted)
		if a.addOrUpdateConfirmation(found, res) {
			a.logMetricAsServer("accept signup")
		}

		res.WriteHeader(http.StatusOK)
	}
}

//In the event that someone uses the wrong email address, the receiver could explicitly dismiss a signup attempt with this link (useful for metrics and to identify phishing attempts).
//This link would be some sort of parenthetical comment in the signup confirmation email, like "(I didn't try to sign up for Tidepool.)"
//No authentication required.
//
// status: 200
// status: 400 STATUS_SIGNUP_NO_ID
// status: 400 STATUS_SIGNUP_NO_CONF
// status: 400 STATUS_ERR_DECODING_CONFIRMATION
// status: 404 STATUS_SIGNUP_NOT_FOUND
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

//This call is provided for completeness -- we don't expect to need it in the actual user flow.
//
//Fetch any existing confirmation requests.
// status: 200 with a single result in an array
// status: 404
func (a *Api) getSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {

		userId := vars["userid"]

		if userId == "" {
			log.Printf("getSignUp %s", STATUS_SIGNUP_NO_ID)
			a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusBadRequest, STATUS_SIGNUP_NO_ID), http.StatusBadRequest)
			return
		}

		if permissions, err := a.tokenUserHasRequestedPermissions(token, userId, commonClients.Permissions{"root": commonClients.Allowed, "custodian": commonClients.Allowed}); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, err)
			return
		} else if permissions["root"] == nil && permissions["custodian"] == nil {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		if signups, _ := a.Store.FindConfirmations(&models.Confirmation{UserId: userId, Type: models.TypeSignUp}, models.StatusPending); signups == nil {
			log.Printf("getSignUp %s", STATUS_SIGNUP_NOT_FOUND)
			a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusNotFound, STATUS_SIGNUP_NOT_FOUND), http.StatusNotFound)
			return
		} else {
			a.logMetric("get signups", req)
			log.Printf("getSignUp found %d for user %s", len(signups), userId)
			a.sendModelAsResWithStatus(res, signups, http.StatusOK)
			return
		}
	}
	return
}

// status: 200
// status: 400 STATUS_SIGNUP_NO_ID
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
