package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"go.uber.org/zap"

	clinics "github.com/tidepool-org/clinic/client"
	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
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
			log.Println(statusExistingInviteMessage)
			log.Println("last invite not yet expired")
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
			log.Printf("error checking if user is in group [%v]", err)
		} else if perms != nil {
			log.Println(statusExistingMemberMessage)
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
			a.logger.Warnf("token owner %s is not authorized to accept invite of for %s", token.UserID, inviteeID)
			a.sendModelAsResWithStatus(res, status.StatusError{Status: status.NewStatus(http.StatusUnauthorized, STATUS_UNAUTHORIZED)}, http.StatusUnauthorized)
			return
		}

		invitedUsr := a.findExistingUser(inviteeID, req.Header.Get(TP_SESSION_TOKEN))

		//find all oustanding invites were this user is the invite//
		found, err := a.Store.FindConfirmations(req.Context(), &models.Confirmation{Email: invitedUsr.Emails[0], Type: models.TypeCareteamInvite}, models.StatusPending)
		if err != nil {
			a.logger.Errorw("error while finding pending invites", zap.Error(err))
		}

		if invites := a.checkFoundConfirmations(res, found, err); invites != nil {
			a.ensureIdSet(req.Context(), inviteeID, invites)
			a.logger.Infof("found and have checked [%d] invites ", len(invites))
			a.logMetric("get received invites", req)
			a.sendModelAsResWithStatus(res, invites, http.StatusOK)
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
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, err)
			return
		} else if permissions["root"] == nil && permissions["custodian"] == nil {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		//find all invites I have sent that are pending or declined
		found, err := a.Store.FindConfirmations(req.Context(), &models.Confirmation{CreatorId: invitorID, Type: models.TypeCareteamInvite}, models.StatusPending, models.StatusDeclined)
		if invitations := a.checkFoundConfirmations(res, found, err); invitations != nil {
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
			log.Printf("AcceptInvite inviteeID %s or invitorID %s not set", inviteeID, invitorID)
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		// Non-server tokens only legit when for same userid
		if !token.IsServer && inviteeID != token.UserID {
			log.Println("AcceptInvite ", STATUS_UNAUTHORIZED)
			a.sendModelAsResWithStatus(
				res,
				status.StatusError{Status: status.NewStatus(http.StatusUnauthorized, STATUS_UNAUTHORIZED)},
				http.StatusUnauthorized,
			)
			return
		}

		accept := &models.Confirmation{}
		if err := json.NewDecoder(req.Body).Decode(accept); err != nil {
			log.Printf("AcceptInvite error decoding invite data: %v\n", err)
			a.sendModelAsResWithStatus(
				res,
				&status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)},
				http.StatusBadRequest,
			)
			return
		}

		if accept.Key == "" {
			log.Println("AcceptInvite has no confirmation key set")
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		conf, err := a.Store.FindConfirmation(req.Context(), accept)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if conf == nil {
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusNotFound, statusInviteNotFoundMessage)}
			log.Println("AcceptInvite ", statusErr.Error())
			a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
			return
		}

		validationErrors := []error{}

		conf.ValidateStatus(models.StatusPending, &validationErrors).
			ValidateType(models.TypeCareteamInvite, &validationErrors).
			ValidateUserID(inviteeID, &validationErrors).
			ValidateCreatorID(invitorID, &validationErrors)

		if len(validationErrors) > 0 {
			for _, validationError := range validationErrors {
				log.Println("AcceptInvite forbidden as there was a expectation mismatch", validationError)
			}
			a.sendModelAsResWithStatus(
				res,
				&status.StatusError{Status: status.NewStatus(http.StatusForbidden, statusForbiddenMessage)},
				http.StatusForbidden,
			)
			return
		}

		ctc := &models.CareTeamContext{}
		if err := conf.DecodeContext(ctc); err != nil {
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONTEXT)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
			return
		}

		if err := ctc.Validate(); err != nil {
			log.Printf("AcceptInvite error validating CareTeamContext: %s", err)
			a.sendModelAsResWithStatus(
				res,
				&status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_VALIDATING_CONTEXT)},
				http.StatusForbidden,
			)
			return
		}

		setPerms, err := a.gatekeeper.SetPermissions(inviteeID, invitorID, ctc.Permissions)
		if err != nil {
			log.Printf("AcceptInvite error setting permissions [%v]", err)
			a.sendModelAsResWithStatus(
				res,
				&status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_DECODING_CONFIRMATION)},
				http.StatusInternalServerError,
			)
			return
		}
		log.Printf("AcceptInvite: permissions were set as [%v] after an invite was accepted", setPerms)
		if ctc.AlertsConfig != nil && ctc.Permissions["follow"] != nil {
			if err := a.alerts.Upsert(req.Context(), ctc.AlertsConfig); err != nil {
				log.Printf("AcceptInvite: error creating alerting config: %s", err)
				a.sendModelAsResWithStatus(
					res,
					&status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_CREATING_ALERTS_CONFIG)},
					http.StatusInternalServerError,
				)
				return
			}
		}
		conf.UpdateStatus(models.StatusCompleted)
		if !a.addOrUpdateConfirmation(req.Context(), conf, res) {
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION)}
			log.Println("AcceptInvite ", statusErr.Error())
			a.sendModelAsResWithStatus(res, statusErr, http.StatusInternalServerError)
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
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, err)
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
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusNotFound, statusInviteNotFoundMessage)}
		log.Printf("CancelInvite: [%s]", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
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
			log.Printf("DismissInvite %s ", STATUS_UNAUTHORIZED)
			a.sendModelAsResWithStatus(res, status.StatusError{Status: status.NewStatus(http.StatusUnauthorized, STATUS_UNAUTHORIZED)}, http.StatusUnauthorized)
			return
		}

		dismiss := &models.Confirmation{}
		if err := json.NewDecoder(req.Body).Decode(dismiss); err != nil {
			log.Printf("DismissInvite: error decoding invite to dismiss [%v]", err)
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
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
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusNotFound, statusInviteNotFoundMessage)}
		log.Printf("DismissInvite: [%s]", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
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
		a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, err)
		return
	} else if permissions["root"] == nil && permissions["custodian"] == nil {
		a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
		return
	}

	var ib = &inviteBody{}
	if err := json.NewDecoder(req.Body).Decode(ib); err != nil {
		log.Printf("SendInvite: error decoding invite to detail %v\n", err)
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
		return
	}

	if ib.Email == "" || ib.Permissions == nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	if a.checkForDuplicateInvite(req.Context(), ib.Email, invitorID) {
		log.Printf("SendInvite: invited [%s] user already has or had an invite", ib.Email)
		statusErr := &status.StatusError{
			Status: status.NewStatus(http.StatusConflict, statusExistingInviteMessage),
		}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
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
			a.logger.Infof("invited [%s] user is already a member of the care team of %v", ib.Email, invitorID)
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusConflict, statusExistingMemberMessage)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
			return
		}
		for key := range perms {
			log.Printf("adding permission: %q %+v", key, perms[key])
			ib.Permissions[key] = perms[key]
		}
	}

	templateName := models.TemplateNameCareteamInvite
	if ib.Permissions["follow"] != nil {
		templateName = models.TemplateNameCareteamInviteWithAlerting
	}

	invite, err := models.NewConfirmationWithContext(models.TypeCareteamInvite, templateName, invitorID, ib.CareTeamContext)
	if err != nil {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, statusInternalServerErrorMessage)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusInternalServerError)
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
		log.Println("SendInvite: ", err.Error())
		a.sendModelAsResWithStatus(res, invite, http.StatusOK)
	}

	fullName := "Tidepool User"
	if invite.Creator.Profile != nil {
		fullName = invite.Creator.Profile.FullName
		if invite.Creator.Profile.Patient.IsOtherPerson {
			fullName = invite.Creator.Profile.Patient.FullName
		}
	}

	var webPath = "signup"

	if invite.UserId != "" {
		webPath = "login"
	}

	emailContent := map[string]interface{}{
		"CareteamName": fullName,
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
				a.logger.Warn("cannot resend clinic invite using care team invite endpoint")
			} else {
				a.logger.Warn("cannot resend confirmation, because it doesn't exist")
			}

			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusForbidden, statusForbiddenMessage)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusForbidden)
			return
		}

		if permissions, err := a.tokenUserHasRequestedPermissions(token, invite.CreatorId, commonClients.Permissions{"root": commonClients.Allowed, "custodian": commonClients.Allowed}); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, err)
			return
		} else if permissions["root"] == nil && permissions["custodian"] == nil {
			a.sendError(res, http.StatusForbidden, statusForbiddenMessage)
			return
		}

		invite.ResetCreationAttributes()
		if a.addOrUpdateConfirmation(req.Context(), invite, res) {
			a.logMetric("invite updated", req)

			if err := a.addProfile(invite); err != nil {
				a.logger.Warn("Resend invite", zap.Error(err))
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
