package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"time"

	"go.uber.org/zap"

	clinics "github.com/tidepool-org/clinic/client"

	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"

	"github.com/tidepool-org/hydrophone/models"
)

// try to find the signup confirmation
func (a *Api) findSignUp(ctx context.Context, conf *models.Confirmation, res http.ResponseWriter) *models.Confirmation {
	found, err := a.Store.FindConfirmation(ctx, conf)
	if err != nil {
		a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
		return nil
	}
	if found == nil {
		a.sendError(res, http.StatusNotFound, STATUS_SIGNUP_NOT_FOUND)
		return nil
	}

	return found
}

// update an existing signup confirmation
func (a *Api) updateSignupConfirmation(newStatus models.Status, res http.ResponseWriter, req *http.Request) {
	fromBody := &models.Confirmation{}
	if err := json.NewDecoder(req.Body).Decode(fromBody); err != nil {
		a.sendError(res, http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION, err)
		return
	}

	if fromBody.Key == "" {
		a.sendError(res, http.StatusBadRequest, STATUS_SIGNUP_NO_CONF)
		return
	}

	found, err := a.Store.FindConfirmation(req.Context(), fromBody)
	if err != nil {
		a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
		return
	}
	if found != nil {
		updatedStatus := string(newStatus) + " signup"
		found.UpdateStatus(newStatus)

		if a.addOrUpdateConfirmation(req.Context(), found, res) {
			a.logMetricAsServer(updatedStatus)
			res.WriteHeader(http.StatusOK)
			return
		}
	} else {
		a.sendError(res, http.StatusNotFound, STATUS_SIGNUP_NOT_FOUND)
		return
	}
}

// Send a signup confirmation email to a userid.
//
// This post is sent by the signup logic. In this state, the user account has been created but has a flag that
// forces the user to the confirmation-required page until the signup has been confirmed.
// It sends an email that contains a random confirmation link.
//
// status: 201
// status: 400 STATUS_SIGNUP_NO_ID
// status: 401 STATUS_NO_TOKEN
// status: 403 STATUS_EXISTING_SIGNUP
// status: 500 STATUS_ERR_FINDING_USER
func (a *Api) sendSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if newSignUp := a.upsertSignUp(res, req, vars); newSignUp != nil {
		clinicName := "Diabetes Clinic"
		creatorName := "Clinician"

		if newSignUp.ClinicId != "" {
			resp, err := a.clinics.GetClinicWithResponse(req.Context(), clinics.ClinicId(newSignUp.ClinicId))
			if err != nil {
				a.sendError(res, http.StatusInternalServerError, "unable to fetch clinic")
				return
			}
			if resp.StatusCode() == http.StatusOK && resp.JSON200.Name != "" {
				clinicName = resp.JSON200.Name
			}
		}

		if err := a.addProfile(newSignUp); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_ADDING_PROFILE, err)
			return
		}
		if newSignUp.Creator.Profile != nil && newSignUp.Creator.Profile.FullName != "" {
			creatorName = newSignUp.Creator.Profile.FullName
		}

		profile := &models.Profile{}
		if err := a.seagull.GetCollection(newSignUp.UserId, "profile", a.sl.TokenProvide(), profile); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, "getting user profile", err)
			return
		}

		a.logger.
			With(zap.String("email", newSignUp.Email)).
			With(zap.String("key", newSignUp.Key)).
			Debug("sending email confirmation")

		emailContent := map[string]interface{}{
			"Key":         newSignUp.Key,
			"Email":       newSignUp.Email,
			"FullName":    profile.FullName,
			"CreatorName": creatorName,
		}
		if newSignUp.ClinicId != "" {
			emailContent["ClinicName"] = clinicName
		}

		if a.createAndSendNotification(req, newSignUp, emailContent) {
			a.logMetricAsServer("signup confirmation sent")
			res.WriteHeader(http.StatusOK)
			return
		} else {
			a.logMetric("signup confirmation failed to be sent", req)
		}
	}
}

// If a user didn't receive the confirmation email and logs in, they're directed to the confirmation-required page which can
// offer to resend the confirmation email.
//
// status: 200
// status: 404 STATUS_SIGNUP_EXPIRED
func (a *Api) resendSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	email := vars["useremail"]

	toFind := &models.Confirmation{Email: email, Status: models.StatusPending, Type: models.TypeSignUp}

	if found := a.findSignUp(req.Context(), toFind, res); found != nil {
		if err := a.Store.RemoveConfirmation(req.Context(), found); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_DELETING_CONFIRMATION, err)
			return
		}

		if err := found.ResetKey(); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_RESETTING_KEY, err)
			return
		}

		if a.addOrUpdateConfirmation(req.Context(), found, res) {
			a.logMetricAsServer("signup confirmation recreated")

			if err := a.addProfile(found); err != nil {
				a.sendError(res, http.StatusInternalServerError, STATUS_ERR_ADDING_PROFILE, err)
				return
			} else {
				profile := &models.Profile{}
				if err := a.seagull.GetCollection(found.UserId, "profile", a.sl.TokenProvide(), profile); err != nil {
					a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, err)
					return
				}

				a.logger.
					With(zap.String("email", found.Email)).
					With(zap.String("key", found.Key)).
					Debug("resending email confirmation")

				emailContent := map[string]interface{}{
					"Key":      found.Key,
					"Email":    found.Email,
					"FullName": profile.FullName,
				}

				if found.Creator.Profile != nil {
					emailContent["CreatorName"] = found.Creator.Profile.FullName
				}

				if found.ClinicId != "" {
					resp, err := a.clinics.GetClinicWithResponse(req.Context(), clinics.ClinicId(found.ClinicId))
					if err != nil {
						a.sendError(res, http.StatusInternalServerError, "unable to fetch clinic")
						return
					}
					if resp.StatusCode() == http.StatusOK && resp.JSON200.Name != "" {
						emailContent["ClinicName"] = resp.JSON200.Name
					}
				}

				if a.createAndSendNotification(req, found, emailContent) {
					a.logMetricAsServer("signup confirmation re-sent")
				} else {
					a.logMetricAsServer("signup confirmation failed to be sent")
				}
			}

			res.WriteHeader(http.StatusOK)
		}
	}
}

// This would be PUT by the web page at the link in the signup email. No authentication is required.
// When this call is made, the flag that prevents login on an account is removed, and the user is directed to the login page.
// If the user has an active cookie for signup (created with a short lifetime) we can accept the presence of that cookie to allow the actual login to be skipped.
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
		a.sendError(res, http.StatusBadRequest, STATUS_SIGNUP_NO_CONF)
		return
	}

	toFind := &models.Confirmation{Key: confirmationId}

	if found := a.findSignUp(req.Context(), toFind, res); found != nil {
		if found.IsExpired() {
			a.sendError(res, http.StatusNotFound, STATUS_SIGNUP_EXPIRED)
			return
		}

		emailVerified := true
		updates := shoreline.UserUpdate{EmailVerified: &emailVerified}

		if user, err := a.sl.GetUser(found.UserId, a.sl.TokenProvide()); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, "trying to get user to check email verified", err)
			return

		} else if !user.PasswordExists {
			acceptance := &models.Acceptance{}
			if req.Body != nil {
				if err := json.NewDecoder(req.Body).Decode(acceptance); err != nil {
					a.sendErrorWithCode(res, http.StatusConflict, ERROR_NO_PASSWORD, STATUS_NO_PASSWORD, "decoding acceptance", err)
					return
				}
			}

			if acceptance.Password == "" {
				a.sendErrorWithCode(res, http.StatusConflict, ERROR_MISSING_PASSWORD, STATUS_MISSING_PASSWORD, "missing password")
				return
			}
			if !IsValidPassword(acceptance.Password) {
				a.sendErrorWithCode(res, http.StatusConflict, ERROR_INVALID_PASSWORD, STATUS_INVALID_PASSWORD, "invalid password specified")
				return
			}
			if acceptance.Birthday == "" {
				a.sendErrorWithCode(res, http.StatusConflict, ERROR_MISSING_BIRTHDAY, STATUS_MISSING_BIRTHDAY, "missing birthday")
				return
			}
			if !IsValidDate(acceptance.Birthday) {
				a.sendErrorWithCode(res, http.StatusConflict, ERROR_INVALID_BIRTHDAY, STATUS_INVALID_BIRTHDAY, "invalid birthday specified")
				return
			}

			profile := &models.Profile{}
			if err := a.seagull.GetCollection(found.UserId, "profile", a.sl.TokenProvide(), profile); err != nil {
				a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, "getting the users profile", err)
				return
			}

			if acceptance.Birthday != profile.Patient.Birthday {
				a.sendErrorWithCode(res, http.StatusConflict, ERROR_MISMATCH_BIRTHDAY, STATUS_MISMATCH_BIRTHDAY, "acceptance birthday does not match user patient birthday")
				return
			}

			updates.Password = &acceptance.Password
		}

		if err := a.sl.UpdateUser(found.UserId, updates, a.sl.TokenProvide()); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_UPDATING_USER, err)
			return
		}

		found.UpdateStatus(models.StatusCompleted)
		if a.addOrUpdateConfirmation(req.Context(), found, res) {
			a.logMetricAsServer("accept signup")
		}

		res.WriteHeader(http.StatusOK)
	}
}

// In the event that someone uses the wrong email address, the receiver could explicitly dismiss a signup attempt with this link (useful for metrics and to identify phishing attempts).
// This link would be some sort of parenthetical comment in the signup confirmation email, like "(I didn't try to sign up for Tidepool.)"
// No authentication required.
//
// status: 200
// status: 400 STATUS_SIGNUP_NO_ID
// status: 400 STATUS_SIGNUP_NO_CONF
// status: 400 STATUS_ERR_DECODING_CONFIRMATION
// status: 404 STATUS_SIGNUP_NOT_FOUND
func (a *Api) dismissSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	userId := vars["userid"]

	if userId == "" {
		a.sendError(res, http.StatusBadRequest, STATUS_SIGNUP_NO_ID)
		return
	}
	a.logger.Debug("dismissing invite")
	a.updateSignupConfirmation(models.StatusDeclined, res, req)
}

// Fetch latest account signup confirmation for the provided userId
//
// status: 200 with a single result in an array
// status: 404
func (a *Api) getSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {

		userId := vars["userid"]

		if userId == "" {
			a.sendError(res, http.StatusBadRequest, STATUS_SIGNUP_NO_ID)
			return
		}

		if permissions, err := a.tokenUserHasRequestedPermissions(token, userId, commonClients.Permissions{"root": commonClients.Allowed, "custodian": commonClients.Allowed}); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, err)
			return
		} else if permissions["root"] == nil && permissions["custodian"] == nil {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		signup, err := a.Store.FindConfirmation(req.Context(), &models.Confirmation{UserId: userId, Type: models.TypeSignUp, Status: models.StatusPending})
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}

		if signup == nil {
			a.sendError(res, http.StatusNotFound, STATUS_SIGNUP_NOT_FOUND)
			return
		} else {
			a.logMetric("get signup", req)
			a.sendModelAsResWithStatus(res, signup, http.StatusOK)
			a.logger.Debug("found a pending signup")
			return
		}
	}
}

// Add or refresh an account signup confirmation for the provided userId
//
// status: 200 newly upserted signup
// status: 400 STATUS_SIGNUP_NO_ID
// status: 401 STATUS_NO_TOKEN
// status: 403 STATUS_EXISTING_SIGNUP
// status: 500 STATUS_ERR_FINDING_USER
func (a *Api) createSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if newSignUp := a.upsertSignUp(res, req, vars); newSignUp != nil {
		a.sendModelAsResWithStatus(res, newSignUp, http.StatusOK)
	}
}

// Return a new or refreshed an account signup confirmation
//
// If it returns nil, an HTTP response has been sent.
//
// status: 400 STATUS_SIGNUP_NO_ID
// status: 401 STATUS_NO_TOKEN
// status: 403 STATUS_EXISTING_SIGNUP
// status: 500 STATUS_ERR_FINDING_USER
func (a *Api) upsertSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) *models.Confirmation {
	if token := a.token(res, req); token != nil {
		userId := vars["userid"]
		if userId == "" {
			a.sendError(res, http.StatusBadRequest, STATUS_SIGNUP_NO_ID)
			return nil
		}

		if permissions, err := a.tokenUserHasRequestedPermissions(token, userId, commonClients.Permissions{"root": commonClients.Allowed, "custodian": commonClients.Allowed}); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, err)
			return nil
		} else if permissions["root"] == nil && permissions["custodian"] == nil {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return nil
		}

		var upsertCustodialSignUpInvite UpsertCustodialSignUpInvite
		if err := json.NewDecoder(req.Body).Decode(&upsertCustodialSignUpInvite); err != nil && err != io.EOF {
			a.sendError(res, http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)
			return nil
		}

		if usrDetails, err := a.sl.GetUser(userId, a.sl.TokenProvide()); err != nil {
			a.sendError(res, http.StatusNotFound, STATUS_ERR_FINDING_USER, err)
			return nil
		} else if len(usrDetails.Emails) == 0 {
			// Delete existing any existing invites if the email address is empty
			existing, err := a.Store.FindConfirmation(req.Context(), &models.Confirmation{UserId: usrDetails.UserID, Type: models.TypeSignUp})
			if err != nil {
				a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
				return nil
			}
			if existing != nil {
				if err := a.Store.RemoveConfirmation(req.Context(), existing); err != nil {
					a.sendError(res, http.StatusInternalServerError, STATUS_ERR_DELETING_CONFIRMATION, err)
					return nil
				}
			}
			res.WriteHeader(http.StatusOK)
			return nil
		} else {

			// get any existing confirmations
			newSignUp, err := a.Store.FindConfirmation(req.Context(), &models.Confirmation{UserId: usrDetails.UserID, Type: models.TypeSignUp})
			if err != nil {
				a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
				return nil
			} else if newSignUp == nil {

				var templateName models.TemplateName
				var creatorID string
				var clinicId string

				if usrDetails.IsClinic() {
					templateName = models.TemplateNameSignupClinic
				} else if usrDetails.IsCustodial() {
					if token.IsServer {
						if upsertCustodialSignUpInvite.ClinicId != "" {
							templateName = models.TemplateNameSignupCustodialNewClinicExperience
							creatorID = upsertCustodialSignUpInvite.InvitedBy
							clinicId = upsertCustodialSignUpInvite.ClinicId
						} else {
							templateName = models.TemplateNameSignupCustodial
						}
					} else {
						tokenUserDetails, err := a.sl.GetUser(token.UserID, a.sl.TokenProvide())
						if err != nil {
							a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, err)
							return nil
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

				newSignUp, err = models.NewConfirmation(models.TypeSignUp, templateName, creatorID)
				if err != nil {
					a.sendError(res, http.StatusInternalServerError, STATUS_ERR_CREATING_CONFIRMATION, err)
					return nil
				}

				newSignUp.UserId = usrDetails.UserID
				newSignUp.Email = usrDetails.Emails[0]
				newSignUp.ClinicId = clinicId
			} else if newSignUp.Email != usrDetails.Emails[0] {

				if err := a.Store.RemoveConfirmation(req.Context(), newSignUp); err != nil {
					a.sendError(res, http.StatusInternalServerError, STATUS_ERR_DELETING_CONFIRMATION, err)
					return nil
				}

				if err := newSignUp.ResetKey(); err != nil {
					a.sendError(res, http.StatusInternalServerError, STATUS_ERR_RESETTING_KEY, err)
					return nil
				}

				newSignUp.Email = usrDetails.Emails[0]
				if upsertCustodialSignUpInvite.ClinicId != "" {
					newSignUp.ClinicId = upsertCustodialSignUpInvite.ClinicId
					newSignUp.CreatorId = upsertCustodialSignUpInvite.InvitedBy
				}
			} else {
				a.sendError(res, http.StatusForbidden, STATUS_EXISTING_SIGNUP)
				return nil
			}

			if a.addOrUpdateConfirmation(req.Context(), newSignUp, res) {
				a.logMetric("signup confirmation created", req)

				return newSignUp
			}
		}
	}
	return nil
}

// status: 200
// status: 400 STATUS_SIGNUP_NO_ID
func (a *Api) cancelSignUp(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	userId := vars["userid"]

	if userId == "" {
		a.sendError(res, http.StatusBadRequest, STATUS_SIGNUP_NO_ID)
		return
	}
	a.updateSignupConfirmation(models.StatusCanceled, res, req)
	a.logger.Debug("canceled signup")
}

func IsValidPassword(password string) bool {
	return passwordRe.MatchString(password)
}

func IsValidDate(date string) bool {
	_, err := time.Parse("2006-01-02", date)
	return err == nil
}

type UpsertCustodialSignUpInvite struct {
	ClinicId  string `json:"clinicId"`
	InvitedBy string `json:"invitedBy"`
}

var passwordRe = regexp.MustCompile(`\A\S{8,72}\z`)
