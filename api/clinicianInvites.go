package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/hydrophone/models"
)

type ClinicianInvite struct {
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

// Send an invite to become a clinic member
func (a *Api) SendClinicianInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		ctx := req.Context()
		clinicId := vars["clinicId"]

		if err := a.assertClinicAdmin(ctx, clinicId, token, res); err != nil {
			// assertClinicAdmin will log and send a response
			return
		}

		clinic, err := a.clinics.GetClinicWithResponse(ctx, clinics.ClinicId(clinicId))
		if err != nil || clinic == nil || clinic.JSON200 == nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CLINIC, err)
			return
		}

		defer req.Body.Close()
		var body = &ClinicianInvite{}
		if err := json.NewDecoder(req.Body).Decode(body); err != nil {
			a.sendError(ctx, res, http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION, err)
			return
		}

		confirmation, err := models.NewConfirmation(models.TypeClinicianInvite, models.TemplateNameClinicianInvite, token.UserID)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_CREATING_CONFIRMATION, err)
			return
		}

		confirmation.Email = body.Email
		confirmation.ClinicId = *clinic.JSON200.Id
		confirmation.Creator.ClinicId = *clinic.JSON200.Id
		confirmation.Creator.ClinicName = clinic.JSON200.Name

		invitedUsr := a.findExistingUser(ctx, body.Email, a.sl.TokenProvide())
		if invitedUsr != nil && invitedUsr.UserID != "" {
			confirmation.UserId = invitedUsr.UserID
		}

		response, err := a.clinics.CreateClinicianWithResponse(ctx, clinics.ClinicId(clinicId), clinics.CreateClinicianJSONRequestBody{
			InviteId: &confirmation.Key,
			Email:    body.Email,
			Roles:    body.Roles,
		})
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CLINIC, err)
			return
		}
		if response.StatusCode() != http.StatusOK {
			res.Header().Set("content-type", "application/json")
			res.WriteHeader(response.StatusCode())
			res.Write(response.Body)
			return
		}

		msg, err := a.sendClinicianConfirmation(req, confirmation)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, msg, err)
			return
		}

		res.Header().Set("content-type", "application/json")
		res.WriteHeader(http.StatusOK)
		res.Write(response.Body)
		return
	}
}

// Resend an invite to become a clinic member
func (a *Api) ResendClinicianInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		ctx := req.Context()
		clinicId := vars["clinicId"]
		inviteId := vars["inviteId"]

		if err := a.assertClinicAdmin(ctx, clinicId, token, res); err != nil {
			// assertClinicAdmin will log and send a response
			return
		}

		clinic, err := a.clinics.GetClinicWithResponse(ctx, clinics.ClinicId(clinicId))
		if err != nil || clinic == nil || clinic.JSON200 == nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CLINIC, err)
			return
		}

		inviteResponse, err := a.clinics.GetInvitedClinicianWithResponse(ctx, clinics.ClinicId(clinicId), clinics.InviteId(inviteId))
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CLINIC, err)
			return
		}
		if inviteResponse.StatusCode() != http.StatusOK || inviteResponse.JSON200 == nil {
			res.Header().Set("content-type", "application/json")
			res.WriteHeader(inviteResponse.StatusCode())
			res.Write(inviteResponse.Body)
			return
		}

		filter := &models.Confirmation{
			Key:    inviteId,
			Type:   models.TypeClinicianInvite,
			Status: models.StatusPending,
		}
		confirmation, err := a.Store.FindConfirmation(ctx, filter)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if confirmation == nil {
			confirmation, err := models.NewConfirmation(models.TypeClinicianInvite, models.TemplateNameClinicianInvite, token.UserID)
			if err != nil {
				a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_CREATING_CONFIRMATION, err)
				return
			}
			confirmation.Key = inviteId
		}

		confirmation.Email = inviteResponse.JSON200.Email
		confirmation.ClinicId = *clinic.JSON200.Id
		confirmation.Creator.ClinicId = *clinic.JSON200.Id
		confirmation.Creator.ClinicName = clinic.JSON200.Name

		invitedUsr := a.findExistingUser(ctx, confirmation.Email, a.sl.TokenProvide())
		if invitedUsr != nil && invitedUsr.UserID != "" {
			confirmation.UserId = invitedUsr.UserID
		}

		msg, err := a.sendClinicianConfirmation(req, confirmation)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, msg, err)
			return
		}

		a.sendModelAsResWithStatus(ctx, res, confirmation, http.StatusOK)
		return
	}
}

// Get an invite to become a clinic member
func (a *Api) GetClinicianInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		ctx := req.Context()
		clinicId := vars["clinicId"]
		inviteId := vars["inviteId"]

		if err := a.assertClinicAdmin(ctx, clinicId, token, res); err != nil {
			// assertClinicAdmin will log and send a response
			return
		}

		// Make sure the invite belongs to the clinic
		inviteResponse, err := a.clinics.GetInvitedClinicianWithResponse(ctx, clinics.ClinicId(clinicId), clinics.InviteId(inviteId))
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CLINIC, err)
			return
		}
		if inviteResponse.StatusCode() != http.StatusOK || inviteResponse.JSON200 == nil {
			res.Header().Set("content-type", "application/json")
			res.WriteHeader(inviteResponse.StatusCode())
			res.Write(inviteResponse.Body)
			return
		}

		filter := &models.Confirmation{
			Key:    inviteId,
			Type:   models.TypeClinicianInvite,
			Status: models.StatusPending,
		}
		confirmation, err := a.Store.FindConfirmation(ctx, filter)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if confirmation == nil {
			a.sendError(ctx, res, http.StatusNotFound, statusInviteNotFoundMessage)
			return
		}

		a.sendModelAsResWithStatus(ctx, res, confirmation, http.StatusOK)
		return
	}
}

// Get the still-pending invitations for a clinician
func (a *Api) GetClinicianInvitations(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		ctx := req.Context()
		userId := vars["userId"]

		invitedUsr := a.findExistingUser(ctx, userId, req.Header.Get(TP_SESSION_TOKEN))

		// Tokens only legit when for same userid
		if userId != token.UserID || invitedUsr == nil || invitedUsr.UserID == "" {
			a.sendError(ctx, res, http.StatusUnauthorized, STATUS_UNAUTHORIZED,
				"token belongs to a different user or user doesn't exist")
			return
		}

		// Populate userId of the confirmations for this user's userId if is not set. This will allow us to query by userId.
		inviteType := models.TypeClinicianInvite
		inviteStatus := models.StatusPending
		if err := a.addUserIdsToUserlessInvites(ctx, invitedUsr, inviteType, inviteStatus); err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_UPDATING_CONFIRMATION, err)
			return
		}

		found, err := a.Store.FindConfirmations(ctx, &models.Confirmation{UserId: invitedUsr.UserID, Type: inviteType}, inviteStatus)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if len(found) == 0 {
			a.sendError(ctx, res, http.StatusNotFound, STATUS_NOT_FOUND)
			return
		}
		if invites := a.addProfileInfoToConfirmations(ctx, found); invites != nil {
			a.ensureIdSet(ctx, userId, invites)
			if err := a.populateRestrictions(ctx, *invitedUsr, *token, invites); err != nil {
				a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err,
					"error populating restriction in invites for user")
				return
			}

			a.logMetric("get_clinician_invitations", req)
			a.sendModelAsResWithStatus(ctx, res, invites, http.StatusOK)
			a.logger(ctx).Infof("invites found and checked: %d", len(invites))
			return
		}
	}
}

// Accept the given invite
func (a *Api) AcceptClinicianInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		ctx := req.Context()
		userId := vars["userId"]
		inviteId := vars["inviteId"]

		invitedUsr := a.findExistingUser(ctx, token.UserID, req.Header.Get(TP_SESSION_TOKEN))

		// Tokens only legit when for same userid
		if token.IsServer || userId != token.UserID || invitedUsr == nil || invitedUsr.UserID != token.UserID {
			a.sendError(ctx, res, http.StatusUnauthorized, STATUS_UNAUTHORIZED,
				"token belongs to a different user or user doesn't exist")
			return
		}

		accept := &models.Confirmation{
			Key:    inviteId,
			UserId: token.UserID,
			Type:   models.TypeClinicianInvite,
			Status: models.StatusPending,
		}

		conf, err := a.Store.FindConfirmation(ctx, accept)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}

		if err := a.populateRestrictions(ctx, *invitedUsr, *token, []*models.Confirmation{conf}); err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err,
				"error populating restriction in invites for uiser")
			return
		}

		if conf.Restrictions != nil && !conf.Restrictions.CanAccept {
			a.sendError(ctx, res, http.StatusForbidden, STATUS_ERR_ACCEPTING_CONFIRMATION)
			return
		}

		association := clinics.AssociateClinicianToUserJSONRequestBody{UserId: token.UserID}
		response, err := a.clinics.AssociateClinicianToUserWithResponse(ctx, clinics.ClinicId(conf.ClinicId), clinics.InviteId(inviteId), association)
		if err != nil || response.StatusCode() != http.StatusOK {
			a.sendModelAsResWithStatus(ctx, res, err, http.StatusInternalServerError)
			return
		}

		conf.UpdateStatus(models.StatusCompleted)
		// addOrUpdateConfirmation logs and writes a response on errors
		if !a.addOrUpdateConfirmation(ctx, conf, res) {
			return
		}

		a.logMetric("accept_clinician_invite", req)
		res.WriteHeader(http.StatusOK)
		res.Write(response.Body)
		return
	}
}

// Dismiss invite
func (a *Api) DismissClinicianInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		ctx := req.Context()
		userId := vars["userId"]
		inviteId := vars["inviteId"]

		invitedUsr := a.findExistingUser(ctx, token.UserID, req.Header.Get(TP_SESSION_TOKEN))
		// Tokens only legit when for same userid
		if token.IsServer || userId != token.UserID || invitedUsr == nil || invitedUsr.UserID != token.UserID {
			a.sendError(ctx, res, http.StatusUnauthorized, STATUS_UNAUTHORIZED,
				"token belongs to a different user or user doesn't exist")
			return
		}

		filter := &models.Confirmation{
			Key:    inviteId,
			UserId: userId,
			Type:   models.TypeClinicianInvite,
			Status: models.StatusPending,
		}
		conf, err := a.Store.FindConfirmation(ctx, filter)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if conf != nil {
			filter.ClinicId = conf.ClinicId
		}

		a.cancelClinicianInviteWithStatus(res, req, filter, conf, models.StatusDeclined)
	}
}

// Cancel invite
func (a *Api) CancelClinicianInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		ctx := req.Context()
		clinicId := vars["clinicId"]
		inviteId := vars["inviteId"]

		if err := a.assertClinicAdmin(ctx, clinicId, token, res); err != nil {
			// assertClinicAdmin will log and send a response
			return
		}

		filter := &models.Confirmation{
			Key:      inviteId,
			ClinicId: clinicId,
			Type:     models.TypeClinicianInvite,
			Status:   models.StatusPending,
		}
		conf, err := a.Store.FindConfirmation(ctx, filter)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}

		a.cancelClinicianInviteWithStatus(res, req, filter, conf, models.StatusCanceled)
	}
}

func (a *Api) sendClinicianConfirmation(req *http.Request, confirmation *models.Confirmation) (msg string, err error) {
	ctx := req.Context()
	if err := a.addProfile(confirmation); err != nil {
		a.logger(ctx).With(zap.Error(err)).Error(STATUS_ERR_ADDING_PROFILE)
		return STATUS_ERR_SAVING_CONFIRMATION, err
	}

	confirmation.Modified = time.Now()
	if err := a.Store.UpsertConfirmation(ctx, confirmation); err != nil {
		return STATUS_ERR_SAVING_CONFIRMATION, err
	}

	a.logMetric("clinician_invite_created", req)

	fullName := confirmation.Creator.Profile.FullName

	var webPath = "signup/clinician"
	if confirmation.UserId != "" {
		webPath = "login"
	}

	emailContent := map[string]interface{}{
		"ClinicName":  confirmation.Creator.ClinicName,
		"CreatorName": fullName,
		"Email":       confirmation.Email,
		"WebPath":     webPath,
	}

	if !a.createAndSendNotification(req, confirmation, emailContent) {
		// TODO: better to re-work createAndSendNotification to return a
		// proper error.
		return STATUS_ERR_SENDING_EMAIL, fmt.Errorf("sending email")
	}

	a.logMetric("clinician_invite_sent", req)
	return "", nil
}

func (a *Api) cancelClinicianInviteWithStatus(res http.ResponseWriter, req *http.Request, filter, conf *models.Confirmation, statusUpdate models.Status) {
	ctx := req.Context()

	response, err := a.clinics.DeleteInvitedClinicianWithResponse(ctx, clinics.ClinicId(filter.ClinicId), clinics.InviteId(filter.Key))
	if err != nil || (response.StatusCode() != http.StatusOK && response.StatusCode() != http.StatusNotFound) {
		a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
		return
	}

	if conf != nil {
		conf.UpdateStatus(statusUpdate)
		// addOrUpdateConfirmation logs and writes a response on errors
		if !a.addOrUpdateConfirmation(ctx, conf, res) {
			return
		}
	}

	a.logMetric("dismiss_clinician_invite", req)
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(STATUS_OK))
	return
}

func (a *Api) assertClinicMember(ctx context.Context, clinicId string, token *shoreline.TokenData, res http.ResponseWriter) error {
	// Non-server tokens only legit when for same userid
	if !token.IsServer {
		if result, err := a.clinics.GetClinicianWithResponse(ctx, clinics.ClinicId(clinicId), clinics.ClinicianId(token.UserID)); err != nil || result.StatusCode() == http.StatusInternalServerError {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, err)
			return err
		} else if result.StatusCode() != http.StatusOK {
			a.sendError(ctx, res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return fmt.Errorf("unexpected status code %v when fetching clinician %v from clinic %v", result.StatusCode(), token.UserID, clinicId)
		}
	}
	return nil
}

func (a *Api) assertClinicAdmin(ctx context.Context, clinicId string, token *shoreline.TokenData, res http.ResponseWriter) error {
	// Non-server tokens only legit when for same userid
	if !token.IsServer {
		if result, err := a.clinics.GetClinicianWithResponse(ctx, clinics.ClinicId(clinicId), clinics.ClinicianId(token.UserID)); err != nil || result.StatusCode() == http.StatusInternalServerError {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, err)
			return err
		} else if result.StatusCode() != http.StatusOK {
			a.sendError(ctx, res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return fmt.Errorf("unexpected status code %v when fetching clinician %v from clinic %v", result.StatusCode(), token.UserID, clinicId)
		} else {
			clinician := result.JSON200
			for _, role := range clinician.Roles {
				if role == "CLINIC_ADMIN" {
					return nil
				}
			}
			a.sendError(ctx, res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return fmt.Errorf("the clinician doesn't have the required permissions %v", clinician.Roles)
		}
	}
	return nil
}
