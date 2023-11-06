package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/hydrophone/models"
)

const (
	//Status message we return from the service
	statusExistingInviteMessage      = "There is already an existing invite"
	statusExistingMemberMessage      = "The user is already an existing member"
	statusExistingPatientMessage     = "The user is already a patient of the clinic"
	statusInviteNotFoundMessage      = "No matching invite was found"
	statusForbiddenMessage           = "Forbidden to perform requested operation"
	statusInternalServerErrorMessage = "Internal Server Error"
)

// Invite details for generating a new invite
type inviteBody struct {
	Email string `json:"email"`
	models.CareTeamContext
	// UnmarshalJSON prevents inviteBody from inheriting it from
	// CareTeamContext.
	UnmarshalJSON struct{} `json:"-"`
}

// Checks do they have an existing invite or are they already a team member
// Or are they an existing user and already in the group?
func (a *Api) checkForDuplicateInvite(ctx context.Context, inviteeEmail, invitorID string) bool {

	//already has invite from this user?
	invites, _ := a.Store.FindConfirmations(
		ctx,
		&models.Confirmation{CreatorId: invitorID, Email: inviteeEmail, Type: models.TypeCareteamInvite},
		models.StatusPending,
	)

	if len(invites) > 0 {

		//rule is we cannot send if the invite is not yet expired
		if !invites[0].IsExpired() {
			a.logger.With(zap.String("email", inviteeEmail)).Debug(statusExistingInviteMessage)
			return true
		}
	}

	return false
}

func (a *Api) checkExistingPatientOfClinic(ctx context.Context, clinicId, patientId string) (bool, error) {
	response, err := a.clinics.GetPatientWithResponse(ctx, clinics.ClinicId(clinicId), clinics.PatientId(patientId))
	if err != nil {
		return false, err
	} else if response.StatusCode() == http.StatusNotFound {
		return false, nil
	} else if response.StatusCode() == http.StatusOK {
		return true, nil
	}

	return false, fmt.Errorf("unexpected status code %v when checking if user is existing patient", response.StatusCode())
}

func (a *Api) checkAccountAlreadySharedWithUser(invitorID, inviteeEmail string) (bool, *shoreline.UserData) {
	//already in the group?
	invitedUsr := a.findExistingUser(inviteeEmail, a.sl.TokenProvide())

	if invitedUsr != nil && invitedUsr.UserID != "" {
		if perms, err := a.gatekeeper.UserInGroup(invitedUsr.UserID, invitorID); err != nil {
			a.logger.With(zap.Error(err)).Error("checking if user is in group")
		} else if perms != nil {
			return true, invitedUsr
		}
		return false, invitedUsr
	}

	return false, nil
}

//Get list of received invitations for logged in user.
//These are invitations that have been sent to this user but not yet acted upon.

// status: 200
// status: 400
func (a *Api) GetReceivedInvitations(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		inviteeID := vars["userid"]

		if inviteeID == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}
		// Non-server tokens only legit when for same userid
		if !token.IsServer && inviteeID != token.UserID {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED,
				zap.String("inviteeID", inviteeID))
			return
		}

		invitedUsr := a.findExistingUser(inviteeID, req.Header.Get(TP_SESSION_TOKEN))

		//find all oustanding invites were this user is the invite//
		found, err := a.Store.FindConfirmations(req.Context(), &models.Confirmation{Email: invitedUsr.Emails[0], Type: models.TypeCareteamInvite}, models.StatusPending)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if len(found) == 0 {
			a.sendError(res, http.StatusNotFound, STATUS_NOT_FOUND)
			return
		}
		if invites := a.addProfileInfoToConfirmations(found); invites != nil {
			a.ensureIdSet(req.Context(), inviteeID, invites)
			a.logMetric("get received invites", req)
			a.sendModelAsResWithStatus(res, invites, http.StatusOK)
			a.logger.Debugf("invites found and checked: %d", len(invites))
			return
		}
	}
}

// Get the still-pending invitations for a group you own or are an admin of.
// These are the invitations you have sent that have not been accepted.
// There is no way to tell if an invitation has been ignored.
//
// status: 200
// status: 400
func (a *Api) GetSentInvitations(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {

		invitorID := vars["userid"]

		if invitorID == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if permissions, err := a.tokenUserHasRequestedPermissions(token, invitorID, commonClients.Permissions{"root": commonClients.Allowed, "custodian": commonClients.Allowed}); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, err)
			return
		} else if permissions["root"] == nil && permissions["custodian"] == nil {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		//find all invites I have sent that are pending or declined
		found, err := a.Store.FindConfirmations(req.Context(), &models.Confirmation{CreatorId: invitorID, Type: models.TypeCareteamInvite}, models.StatusPending, models.StatusDeclined)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if len(found) == 0 {
			a.sendError(res, http.StatusNotFound, STATUS_NOT_FOUND)
			return
		}
		if invitations := a.addProfileInfoToConfirmations(found); invitations != nil {
			a.logMetric("get sent invites", req)
			a.sendModelAsResWithStatus(res, invitations, http.StatusOK)
			return
		}
	}
}

// Accept the given invite
//
// http.StatusOK when accepted
// http.StatusBadRequest when the incoming data is incomplete or incorrect
// http.StatusForbidden when mismatch of user ID's, type or status
func (a *Api) AcceptInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {

		inviteeID := vars["userid"]
		invitorID := vars["invitedby"]

		if inviteeID == "" || invitorID == "" {
			res.WriteHeader(http.StatusBadRequest)
			a.logger.
				With(zap.String("inviteeID", inviteeID)).
				With(zap.String("invitorID", invitorID)).
				Info("inviteeID or invitorID is not set")
			return
		}

		// Non-server tokens only legit when for same userid
		if !token.IsServer && inviteeID != token.UserID {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		accept := &models.Confirmation{}
		if err := json.NewDecoder(req.Body).Decode(accept); err != nil {
			a.sendError(res, http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION, err)
			return
		}

		if accept.Key == "" {
			res.WriteHeader(http.StatusBadRequest)
			a.logger.Info("no confirmation key set")
			return
		}

		conf, err := a.Store.FindConfirmation(req.Context(), accept)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if conf == nil {
			a.sendError(res, http.StatusNotFound, statusInviteNotFoundMessage)
			return
		}

		validationErrors := []error{}

		conf.ValidateStatus(models.StatusPending, &validationErrors).
			ValidateType(models.TypeCareteamInvite, &validationErrors).
			ValidateUserID(inviteeID, &validationErrors).
			ValidateCreatorID(invitorID, &validationErrors)

		if len(validationErrors) > 0 {
			a.sendError(res, http.StatusForbidden, statusForbiddenMessage,
				zap.Errors("validation-errors", validationErrors))
			return
		}

		ctc := &models.CareTeamContext{}
		if err := conf.DecodeContext(ctc); err != nil {
			a.sendError(res, http.StatusBadRequest, STATUS_ERR_DECODING_CONTEXT)
			return
		}

		if err := ctc.Validate(); err != nil {
			a.sendError(res, http.StatusBadRequest, STATUS_ERR_VALIDATING_CONTEXT, err)
			return
		}

		setPerms, err := a.gatekeeper.SetPermissions(inviteeID, invitorID, ctc.Permissions)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_SETTING_PERMISSIONS, err)
			return
		}
		a.logger.With(zapPermsField(setPerms)).Info("permissions set")
		if ctc.AlertsConfig != nil && ctc.Permissions["follow"] != nil {
			if err := a.alerts.Upsert(req.Context(), ctc.AlertsConfig); err != nil {
				a.sendError(res, http.StatusInternalServerError, STATUS_ERR_CREATING_ALERTS_CONFIG, err)
				return
			}
		}
		conf.UpdateStatus(models.StatusCompleted)
		if !a.addOrUpdateConfirmation(req.Context(), conf, res) {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION, err)
			return
		}
		a.logMetric("acceptinvite", req)
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(STATUS_OK))
		return
	}
}

// Cancel an invite the has been sent to an email address
//
// status: 200 when cancled
// status: 404 statusInviteNotFoundMessage
// status: 400 when the incoming data is incomplete or incorrect
func (a *Api) CancelInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {

		invitorID := vars["userid"]
		email := vars["invited_address"]

		if invitorID == "" || email == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if permissions, err := a.tokenUserHasRequestedPermissions(token, invitorID, commonClients.Permissions{"root": commonClients.Allowed, "custodian": commonClients.Allowed}); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, err)
			return
		} else if permissions["root"] == nil && permissions["custodian"] == nil {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		invite := &models.Confirmation{
			Email:     email,
			CreatorId: invitorID,
			Creator:   models.Creator{},
			Type:      models.TypeCareteamInvite,
		}

		conf, err := a.Store.FindConfirmation(req.Context(), invite)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if conf != nil {
			//cancel the invite
			conf.UpdateStatus(models.StatusCanceled)

			if a.addOrUpdateConfirmation(req.Context(), conf, res) {
				a.logMetric("canceled invite", req)
				res.WriteHeader(http.StatusOK)
				return
			}
		}
		a.sendError(res, http.StatusNotFound, statusInviteNotFoundMessage)
		return
	}
}

// status: 200
// status: 400
func (a *Api) DismissInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {

		inviteeID := vars["userid"]
		invitorID := vars["invitedby"]

		if inviteeID == "" || invitorID == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		// Non-server tokens only legit when for same userid
		if !token.IsServer && inviteeID != token.UserID {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		dismiss := &models.Confirmation{}
		if err := json.NewDecoder(req.Body).Decode(dismiss); err != nil {
			a.sendError(res, http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION, err)
			return
		}

		if dismiss.Key == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		conf, err := a.Store.FindConfirmation(req.Context(), dismiss)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if conf != nil {
			conf.UpdateStatus(models.StatusDeclined)

			if a.addOrUpdateConfirmation(req.Context(), conf, res) {
				a.logMetric("dismissinvite", req)
				res.WriteHeader(http.StatusOK)
				return
			}
		}
		a.sendError(res, http.StatusNotFound, statusInviteNotFoundMessage)
		return
	}
}

// Send a invite to join my team
//
// status: 200 models.Confirmation
// status: 409 statusExistingInviteMessage - user already has a pending or declined invite
// status: 409 statusExistingMemberMessage - user is already part of the team
// status: 400
func (a *Api) SendInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	token := a.token(res, req) // a.token writes a response on failure
	if token == nil {
		return
	}

	invitorID := vars["userid"]
	if invitorID == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	requiredPerms := commonClients.Permissions{
		"root":      commonClients.Allowed,
		"custodian": commonClients.Allowed,
	}
	permissions, err := a.tokenUserHasRequestedPermissions(token, invitorID, requiredPerms)
	if err != nil {
		a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, err)
		return
	} else if permissions["root"] == nil && permissions["custodian"] == nil {
		a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
		return
	}

	var ib = &inviteBody{}
	if err := json.NewDecoder(req.Body).Decode(ib); err != nil {
		a.sendError(res, http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION, err)
		return
	}

	if ib.Email == "" || ib.Permissions == nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	if a.checkForDuplicateInvite(req.Context(), ib.Email, invitorID) {
		a.sendError(res, http.StatusConflict, statusExistingInviteMessage,
			zap.String("email", ib.Email))
		return
	}
	alreadyMember, invitedUsr := a.checkAccountAlreadySharedWithUser(invitorID, ib.Email)
	if alreadyMember && invitedUsr != nil {
		// In the past, having an existing relationship would cause this
		// handler to abort with an error response. With the development of
		// the Care Team Alerting features, users with existing relationships
		// should be able to send a new invite that adds alerting
		// permissions. As a result, this code now checks if the current
		// invitation would add alerting permissions, and if so, allows it to
		// continue.
		perms, err := a.gatekeeper.UserInGroup(invitedUsr.UserID, invitorID)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, statusInternalServerErrorMessage)
			return
		}
		if !addsAlertingPermissions(perms, ib.Permissions) {
			// Since this invitation doesn't add alerting permissions,
			// maintain the previous handler's behavior, and abort with an
			// error response.
			a.sendError(res, http.StatusConflict, statusExistingMemberMessage,
				zap.String("email", ib.Email), zap.String("invitorID", invitorID))
			return
		}

		for key := range perms {
			ib.Permissions[key] = perms[key]
		}
		a.logger.With(zapPermsField(perms)).Info("permissions set")
	}

	templateName := models.TemplateNameCareteamInvite
	if ib.Permissions["follow"] != nil {
		templateName = models.TemplateNameCareteamInviteWithAlerting
	}

	invite, err := models.NewConfirmationWithContext(models.TypeCareteamInvite, templateName, invitorID, ib.CareTeamContext)
	if err != nil {
		a.sendError(res, http.StatusInternalServerError, STATUS_ERR_CREATING_CONFIRMATION, err)
		return
	}

	invite.Email = ib.Email
	if invitedUsr != nil {
		invite.UserId = invitedUsr.UserID
	}

	if !a.addOrUpdateConfirmation(req.Context(), invite, res) {
		return
	}
	a.logMetric("invite created", req)

	if err := a.addProfile(invite); err != nil {
		a.logger.With(zap.Error(err)).Warn(STATUS_ERR_ADDING_PROFILE)
		a.sendModelAsResWithStatus(res, invite, http.StatusOK)
	}

	var webPath = "signup"

	if invite.UserId != "" {
		webPath = "login"
	}

	emailContent := map[string]interface{}{
		"CareteamName": findFullname(invite.Creator.Profile),
		"Email":        invite.Email,
		"WebPath":      webPath,
		"Nickname":     ib.Nickname,
	}

	if a.createAndSendNotification(req, invite, emailContent) {
		a.logMetric("invite sent", req)
	}

	a.sendModelAsResWithStatus(res, invite, http.StatusOK)
	return
}

// findFullname of an confirmation's creator, if known.
//
// It falls back to a reasonable generic name if unknown.
func findFullname(profile *models.Profile) string {
	fullname := "Tidepool User"
	if profile != nil {
		if profile.Patient.IsOtherPerson && profile.Patient.FullName != "" {
			fullname = profile.Patient.FullName
		} else if profile.FullName != "" {
			fullname = profile.FullName
		}
	}
	return fullname
}

func addsAlertingPermissions(existingPerms, newPerms commonClients.Permissions) bool {
	return existingPerms["follow"] == nil && newPerms["follow"] != nil
}

// Resend a care team invite
func (a *Api) ResendInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		inviteId := vars["inviteId"]

		if inviteId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		find := &models.Confirmation{
			Key:    inviteId,
			Status: models.StatusPending,
			Type:   models.TypeCareteamInvite,
		}

		invite, err := a.Store.FindConfirmation(req.Context(), find)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if invite == nil || invite.ClinicId != "" {
			if invite.ClinicId != "" {
				a.logger.Info("cannot resend clinic invite using care team invite endpoint")
			} else {
				a.logger.Info("cannot resend confirmation, because it doesn't exist")
			}
			a.sendError(res, http.StatusForbidden, statusForbiddenMessage)
			return
		}

		if permissions, err := a.tokenUserHasRequestedPermissions(token, invite.CreatorId, commonClients.Permissions{"root": commonClients.Allowed, "custodian": commonClients.Allowed}); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, err)
			return
		} else if permissions["root"] == nil && permissions["custodian"] == nil {
			a.sendError(res, http.StatusForbidden, statusForbiddenMessage)
			return
		}

		invite.ResetCreationAttributes()
		if a.addOrUpdateConfirmation(req.Context(), invite, res) {
			a.logMetric("invite updated", req)

			if err := a.addProfile(invite); err != nil {
				a.logger.With(zap.Error(err)).Warn(STATUS_ERR_ADDING_PROFILE)
			} else {
				fullName := invite.Creator.Profile.FullName
				if invite.Creator.Profile.Patient.IsOtherPerson {
					fullName = invite.Creator.Profile.Patient.FullName
				}

				var webPath = "signup"
				if invite.UserId != "" {
					webPath = "login"
				}

				emailContent := map[string]interface{}{
					"CareteamName": fullName,
					"Email":        invite.Email,
					"WebPath":      webPath,
				}

				if a.createAndSendNotification(req, invite, emailContent) {
					a.logMetric("invite resent", req)
				}
			}

			a.sendModelAsResWithStatus(res, invite, http.StatusOK)
			return
		}
	}
}

func zapPermsField(perms clients.Permissions) zap.Field {
	permsForLog := []string{}
	for key := range perms {
		permsForLog = append(permsForLog, key)
	}
	return zap.Strings("perms", permsForLog)
}
