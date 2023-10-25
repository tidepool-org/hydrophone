package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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
			return
		}

		clinic, err := a.clinics.GetClinicWithResponse(ctx, clinics.ClinicId(clinicId))
		if err != nil || clinic == nil || clinic.JSON200 == nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CLINIC, err)
			return
		}

		defer req.Body.Close()
		var body = &ClinicianInvite{}
		if err := json.NewDecoder(req.Body).Decode(body); err != nil {
			a.sendError(res, http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION, err)
			return
		}

		confirmation, err := models.NewConfirmation(models.TypeClinicianInvite, models.TemplateNameClinicianInvite, token.UserID)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_CREATING_CONFIRMATION, err)
			return
		}

		confirmation.Email = body.Email
		confirmation.ClinicId = *clinic.JSON200.Id
		confirmation.Creator.ClinicId = *clinic.JSON200.Id
		confirmation.Creator.ClinicName = clinic.JSON200.Name

		invitedUsr := a.findExistingUser(body.Email, a.sl.TokenProvide())
		if invitedUsr != nil && invitedUsr.UserID != "" {
			confirmation.UserId = invitedUsr.UserID
		}

		response, err := a.clinics.CreateClinicianWithResponse(ctx, clinics.ClinicId(clinicId), clinics.CreateClinicianJSONRequestBody{
			InviteId: &confirmation.Key,
			Email:    body.Email,
			Roles:    body.Roles,
		})
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CLINIC, err)
			return
		}
		if response.StatusCode() != http.StatusOK {
			res.Header().Set("content-type", "application/json")
			res.WriteHeader(response.StatusCode())
			res.Write(response.Body)
			return
		}

		code, msg, optionalErr := a.sendClinicianConfirmation(req, confirmation)
		if code != 0 {
			a.sendError(res, code, msg, optionalErr)
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
			return
		}

		clinic, err := a.clinics.GetClinicWithResponse(ctx, clinics.ClinicId(clinicId))
		if err != nil || clinic == nil || clinic.JSON200 == nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CLINIC, err)
			return
		}

		inviteResponse, err := a.clinics.GetInvitedClinicianWithResponse(ctx, clinics.ClinicId(clinicId), clinics.InviteId(inviteId))
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CLINIC, err)
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
		confirmation, err := a.Store.FindConfirmation(req.Context(), filter)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if confirmation == nil {
			confirmation, err := models.NewConfirmation(models.TypeClinicianInvite, models.TemplateNameClinicianInvite, token.UserID)
			if err != nil {
				a.sendError(res, http.StatusInternalServerError, STATUS_ERR_CREATING_CONFIRMATION, err)
				return
			}
			confirmation.Key = inviteId
		}

		confirmation.Email = inviteResponse.JSON200.Email
		confirmation.ClinicId = *clinic.JSON200.Id
		confirmation.Creator.ClinicId = *clinic.JSON200.Id
		confirmation.Creator.ClinicName = clinic.JSON200.Name

		invitedUsr := a.findExistingUser(confirmation.Email, a.sl.TokenProvide())
		if invitedUsr != nil && invitedUsr.UserID != "" {
			confirmation.UserId = invitedUsr.UserID
		}

		code, msg, optionalErr := a.sendClinicianConfirmation(req, confirmation)
		if code > 0 {
			a.sendError(res, code, msg, optionalErr)
			return
		}

		a.sendModelAsResWithStatus(res, confirmation, http.StatusOK)
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
			return
		}

		// Make sure the invite belongs to the clinic
		inviteResponse, err := a.clinics.GetInvitedClinicianWithResponse(ctx, clinics.ClinicId(clinicId), clinics.InviteId(inviteId))
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CLINIC, err)
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
		confirmation, err := a.Store.FindConfirmation(req.Context(), filter)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if confirmation == nil {
			a.sendError(res, http.StatusNotFound, statusInviteNotFoundMessage)
			return
		}

		a.sendModelAsResWithStatus(res, confirmation, http.StatusOK)
		return
	}
}

// Get the still-pending invitations for a clinician
func (a *Api) GetClinicianInvitations(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		ctx := req.Context()
		userId := vars["userId"]

		invitedUsr := a.findExistingUser(userId, req.Header.Get(TP_SESSION_TOKEN))

		// Tokens only legit when for same userid
		if userId != token.UserID || invitedUsr == nil || invitedUsr.UserID == "" {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		found, err := a.Store.FindConfirmations(ctx, &models.Confirmation{Email: invitedUsr.Emails[0], Type: models.TypeClinicianInvite}, models.StatusPending)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if len(found) == 0 {
			a.sendError(res, http.StatusNotFound, STATUS_NOT_FOUND)
			return
		}
		if invites := a.addProfileInfoToConfirmations(found); invites != nil {
			a.ensureIdSet(req.Context(), userId, invites)
			if err := a.populateRestrictions(ctx, *invitedUsr, *token, invites); err != nil {
				a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
				return
			}

			a.logMetric("get_clinician_invitations", req)
			a.sendModelAsResWithStatus(res, invites, http.StatusOK)
			a.logger.Infof("invites found and checked: %d", len(invites))
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

		invitedUsr := a.findExistingUser(token.UserID, req.Header.Get(TP_SESSION_TOKEN))

		// Tokens only legit when for same userid
		if token.IsServer || userId != token.UserID || invitedUsr == nil || invitedUsr.UserID != token.UserID {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		accept := &models.Confirmation{
			Key:    inviteId,
			UserId: token.UserID,
			Type:   models.TypeClinicianInvite,
			Status: models.StatusPending,
		}

		conf, err := a.Store.FindConfirmation(req.Context(), accept)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}

		if err := a.populateRestrictions(ctx, *invitedUsr, *token, []*models.Confirmation{conf}); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}

		if conf.Restrictions != nil && !conf.Restrictions.CanAccept {
			a.sendError(res, http.StatusForbidden, STATUS_ERR_ACCEPTING_CONFIRMATION)
			return
		}

		association := clinics.AssociateClinicianToUserJSONRequestBody{UserId: token.UserID}
		response, err := a.clinics.AssociateClinicianToUserWithResponse(ctx, clinics.ClinicId(conf.ClinicId), clinics.InviteId(inviteId), association)
		if err != nil || response.StatusCode() != http.StatusOK {
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
			return
		}

		conf.UpdateStatus(models.StatusCompleted)
		if !a.addOrUpdateConfirmation(req.Context(), conf, res) {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION, err)
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

		invitedUsr := a.findExistingUser(token.UserID, req.Header.Get(TP_SESSION_TOKEN))
		// Tokens only legit when for same userid
		if token.IsServer || userId != token.UserID || invitedUsr == nil || invitedUsr.UserID != token.UserID {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
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
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
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
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}

		a.cancelClinicianInviteWithStatus(res, req, filter, conf, models.StatusCanceled)
	}
}

func (a *Api) sendClinicianConfirmation(req *http.Request, confirmation *models.Confirmation) (code int, msg string, err error) {
	ctx := req.Context()

	if err := a.addProfile(confirmation); err != nil {
		return http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION, err
	}

	confirmation.Modified = time.Now()
	if err := a.Store.UpsertConfirmation(ctx, confirmation); err != nil {
		return http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION, err
	}

	a.logMetric("clinician_invite_created", req)

	fullName := confirmation.Creator.Profile.FullName

	var webPath = "signup"
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
		return http.StatusInternalServerError, STATUS_ERR_SENDING_EMAIL, nil
	}

	a.logMetric("clinician_invite_sent", req)
	return 0, "", nil
}

func (a *Api) cancelClinicianInviteWithStatus(res http.ResponseWriter, req *http.Request, filter, conf *models.Confirmation, statusUpdate models.Status) {
	ctx := req.Context()

	response, err := a.clinics.DeleteInvitedClinicianWithResponse(ctx, clinics.ClinicId(filter.ClinicId), clinics.InviteId(filter.Key))
	if err != nil || (response.StatusCode() != http.StatusOK && response.StatusCode() != http.StatusNotFound) {
		a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
		return
	}

	if conf != nil {
		conf.UpdateStatus(statusUpdate)
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
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, err)
			return err
		} else if result.StatusCode() != http.StatusOK {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return fmt.Errorf("unexpected status code %v when fetching clinician %v from clinic %v", result.StatusCode(), token.UserID, clinicId)
		}
	}
	return nil
}

func (a *Api) assertClinicAdmin(ctx context.Context, clinicId string, token *shoreline.TokenData, res http.ResponseWriter) error {
	// Non-server tokens only legit when for same userid
	if !token.IsServer {
		if result, err := a.clinics.GetClinicianWithResponse(ctx, clinics.ClinicId(clinicId), clinics.ClinicianId(token.UserID)); err != nil || result.StatusCode() == http.StatusInternalServerError {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, err)
			return err
		} else if result.StatusCode() != http.StatusOK {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return fmt.Errorf("unexpected status code %v when fetching clinician %v from clinic %v", result.StatusCode(), token.UserID, clinicId)
		} else {
			clinician := result.JSON200
			for _, role := range clinician.Roles {
				if role == "CLINIC_ADMIN" {
					return nil
				}
			}
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return fmt.Errorf("the clinician doesn't have the required permissions %v", clinician.Roles)
		}
	}
	return nil
}
