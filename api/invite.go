package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/mdblp/go-common/v2/clients/status"

	"github.com/mdblp/hydrophone/models"
)

const (
	//Status message we return from the service
	statusExistingInviteMessage  = "There is already an existing invite"
	statusExistingMemberMessage  = "The user is already an existing member"
	statusInviteNotFoundMessage  = "No matching invite was found"
	statusInviteCanceledMessage  = "Invite has been canceled"
	statusInviteNotActiveMessage = "Invite already canceled"
	statusForbiddenMessage       = "Forbidden to perform requested operation"
	statusExpiredMessage         = "Invite has expired"
)

type (
	//Invite details for generating a new invite
	InviteBody struct {
		Email  string `json:"email"`
		TeamID string `json:"teamId"`
		Role   string `json:"role"`
	}
)

func handlerGood(s string) string {
	escapedString := strings.Replace(s, "\n", "", -1)
	escapedString = strings.Replace(escapedString, "\r", "", -1)
	return escapedString
}

func checkInviteBody(ib *InviteBody) *InviteBody {
	var out = &InviteBody{
		Email:  handlerGood(ib.Email),
		Role:   handlerGood(ib.Role),
		TeamID: handlerGood(ib.TeamID),
	}
	return out
}

// Checks do they have an existing invite or are they already a team member
// Or are they an existing user and already in the group?
func (a *Api) checkForDuplicateInvite(ctx context.Context, inviteeEmail, invitorID, token string, res http.ResponseWriter) (bool, *models.UserData) {

	//already has invite from this user?
	invites, _ := a.Store.FindConfirmations(
		ctx,
		&models.Confirmation{CreatorId: invitorID, Email: inviteeEmail, Type: models.TypeCareteamInvite},
		[]models.Status{models.StatusPending},
		[]models.Type{},
	)

	if len(invites) > 0 {

		//rule is we cannot send if the invite is not yet expired
		if !invites[0].IsExpired() {
			log.Println(statusExistingInviteMessage)
			log.Println("last invite not yet expired")
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusConflict, statusExistingInviteMessage)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
			return true, nil
		}
	}

	invitedUsr := a.findExistingUser(inviteeEmail, a.sl.TokenProvide())

	if invitedUsr != nil && invitedUsr.UserID != "" {
		if shares, err := a.perms.GetDirectShares(token); err != nil {
			log.Printf("error checking if user is in group [%v]", err)
		} else if len(shares) > 0 {
			for _, share := range shares {
				if share.ViewerId == invitedUsr.UserID {
					//already sharing data with this user:
					log.Println(statusExistingMemberMessage)
					statusErr := &status.StatusError{Status: status.NewStatus(http.StatusConflict, statusExistingMemberMessage)}
					a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
					return true, invitedUsr
				}
			}

		}
		return false, invitedUsr
	}
	return false, nil
}

// @Summary Get list of received invitations for logged-in user
// @Description  Get list of received invitations that have been sent to this user but not yet acted upon.
// @ID hydrophone-api-GetReceivedInvitations
// @Accept  json
// @Produce  json
// @Param userid path string true "user id"
// @Success 200 {array} models.Confirmation
// @Failure 400 {object} status.Status "usereid was not provided"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provided sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 404 {object} status.Status "no invitations found for this user"
// @Failure 500 {object} status.Status "Error while extracting the data"
// @Router /invitations/{userid} [get]
// @security TidepoolAuth
func (a *Api) GetReceivedInvitations(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		inviteeID := vars["userid"]

		if inviteeID == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}
		// Non-server tokens only legit when for same userid
		if !token.IsServer && inviteeID != token.UserId {
			log.Printf("GetReceivedInvitations %s ", STATUS_UNAUTHORIZED)
			a.sendModelAsResWithStatus(res, status.StatusError{status.NewStatus(http.StatusUnauthorized, STATUS_UNAUTHORIZED)}, http.StatusUnauthorized)
			return
		}

		invitedUsr := a.findExistingUser(inviteeID, a.sl.TokenProvide())

		types := []models.Type{}
		// Show invites relevant for the type of user
		if invitedUsr.HasRole("patient") {
			types = append(types,
				models.TypeMedicalTeamPatientInvite,
			)
		}
		if invitedUsr.HasRole("caregiver") {
			types = append(types, models.TypeCareteamInvite, models.TypeMedicalTeamInvite)
		}
		if invitedUsr.HasRole("hcp") {
			types = append(types,
				models.TypeCareteamInvite,
				models.TypeMedicalTeamInvite,
				models.TypeMedicalTeamDoAdmin,
				models.TypeMedicalTeamRemove,
			)
		}
		status := []models.Status{
			models.StatusPending,
		}
		//find all outstanding invites were this user is the invite//
		found, err := a.Store.FindConfirmations(
			req.Context(),
			&models.Confirmation{Email: invitedUsr.Emails[0]},
			status,
			types,
		)
		// get the team information
		// TODO add below fields under target
		// teamId
		// Name

		//log.Printf("GetReceivedInvitations: found [%d] pending invite(s)", len(found))
		if err != nil {
			log.Printf("GetReceivedInvitations: error [%v] when finding pending invites ", err)
		}

		if invites := a.checkFoundConfirmations(req.Context(), res, found, err); invites != nil {
			a.ensureIdSet(req.Context(), inviteeID, invites)
			log.Printf("GetReceivedInvitations: found and have checked [%d] invites ", len(invites))
			a.logAudit(req, "get received invites")
			a.sendModelAsResWithStatus(res, invites, http.StatusOK)
		}
	}
}

// @Summary Get patient pending invite for a given team
// @Description Get patient pending invite for a given team.
// @ID hydrophone-api-GetPatientTeamPendingInvite
// @Accept  json
// @Produce  json
// @Param teamId path string true "team id"
// @Param patientId path string true "patient id"
// @Success 200 {array} models.Confirmation
// @Failure 400 {object} status.Status "teamId or patientId was not provided"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provided sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 404 {object} status.Status "no invitations found for this user"
// @Failure 500 {object} status.Status "Error while extracting the data"
// @Router /teams/{teamId}/patients/{patientId}/invite [get]
// @security TidepoolAuth
func (a *Api) GetPatientTeamPendingInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	tokenValue := getSessionToken(req)
	token := a.token(res, req)
	if token == nil {
		return
	}

	teamID := vars["teamId"]
	patientID := vars["patientId"]

	_, _, err := a.getTeamForUser(req.Context(), tokenValue, teamID, token.UserId, res)

	if err != nil {
		return
	}

	inviteToFind := models.Confirmation{
		Type:   models.TypeMedicalTeamPatientInvite,
		UserId: patientID,
		Status: models.StatusPending,
		Team: &models.Team{
			ID: teamID,
		},
	}

	var pendingInvite *models.Confirmation
	pendingInvite, err = a.Store.FindConfirmation(req.Context(), &inviteToFind)
	if err != nil {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_FINDING_INVITE)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}

	if pendingInvite == nil {
		res.WriteHeader(http.StatusNotFound)
		return
	}

	a.sendModelAsResWithStatus(res, pendingInvite, http.StatusOK)
	return
}

// @Summary Get the still-pending invitations for a group you own or are an admin of
// @Description  Get list of invitations you have sent that have not been accepted.
// @Description  There is no way to tell if an invitation has been ignored.
// @ID hydrophone-api-GetSentInvitations
// @Accept  json
// @Produce  json
// @Param userid path string true "user id"
// @Success 200 {array} models.Confirmation
// @Failure 400 {object} status.Status "usereid was not provided"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provided sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 404 {object} status.Status "no invitations found for this user"
// @Failure 500 {object} status.Status "Error while extracting the data"
// @Router /invite/{userid} [get]
// @security TidepoolAuth
func (a *Api) GetSentInvitations(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	token := a.token(res, req)
	if token == nil {
		return
	}
	invitorID := vars["userid"]

	if invitorID == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	if !a.isAuthorizedUser(token, invitorID) {
		a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
		return
	}

	//find all invites I have sent that are pending or declined
	found, err := a.Store.FindConfirmations(
		req.Context(),
		&models.Confirmation{
			CreatorId: invitorID,
		},
		[]models.Status{
			models.StatusPending,
		},
		[]models.Type{
			models.TypeCareteamInvite,
			models.TypeMedicalTeamInvite,
			models.TypeMedicalTeamPatientInvite,
		},
	)
	if invitations := a.checkFoundConfirmations(req.Context(), res, found, err); invitations != nil {
		a.logAudit(req, "get sent invites")
		a.sendModelAsResWithStatus(res, invitations, http.StatusOK)
		return
	}
}

// Accept an invite to access patient data
//
// http.StatusOK when accepted
// http.StatusBadRequest when the incoming data is incomplete or incorrect
// http.StatusForbidden when mismatch of user ID's, type or status
// @Summary Accept the given invite
// @Description  This would be PUT by the web page at the link in the invite email. No authentication is required.
// @ID hydrophone-api-acceptInvite
// @Accept  json
// @Produce  json
// @Param userid path string true "invitee id"
// @Param invitedby path string true "invitor id"
// @Param invitation body models.Confirmation true "invitation details"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "inviteeid, invitorid or/and the payload is missing or malformed"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provided sufficient privileges"
// @Failure 403 {object} status.Status "Operation is forbiden. Either the authorization token is invalid or this invite cannot be accepted"
// @Failure 404 {object} status.Status "invitation not found"
// @Failure 500 {object} status.Status "Error (internal) while processing the data"
// @Router /accept/invite/{userid}/{invitedby} [put]
// @security TidepoolAuth
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
		if !token.IsServer && inviteeID != token.UserId {
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

		conf, err := a.findExistingConfirmation(req.Context(), accept, res)
		if err != nil {
			log.Printf("AcceptInvite error while finding confirmation [%s]\n", err.Error())
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
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
			ValidateType([]models.Type{models.TypeCareteamInvite}, &validationErrors).
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

		if err := a.perms.SetPermissions(a.sl.TokenProvide(), invitorID, inviteeID); err != nil {
			log.Printf("AcceptInvite error setting permissions [%v]\n", err)
			a.sendModelAsResWithStatus(
				res,
				&status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_DECODING_CONFIRMATION)},
				http.StatusInternalServerError,
			)
			return
		}
		log.Printf("AcceptInvite: permissions were set for [%v -> %v] after an invite was accepted", invitorID, inviteeID)
		conf.UpdateStatus(models.StatusCompleted)
		if !a.addOrUpdateConfirmation(req.Context(), conf, res) {
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION)}
			log.Println("AcceptInvite ", statusErr.Error())
			a.sendModelAsResWithStatus(res, statusErr, http.StatusInternalServerError)
			return
		}
		a.logAudit(req, "acceptinvite")
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(STATUS_OK))
		return
	}
}

// @Summary Cancel an invite
// @Description Cancel an invite that has been sent to an email address
// @ID hydrophone-api-cancelInvite
// @Accept  json
// @Produce  json
// @Param userid path string true "invitor id"
// @Param invited_address path string true "invited email address"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "invited_address and/or invitorid is missing or incorrect"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provided sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 404 {object} status.Status "invitation not found"
// @Failure 500 {object} status.Status "Error (internal) while processing the data"
// @Router /{userid}/invited/{invited_address} [put]
// @security TidepoolAuth
func (a *Api) CancelInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {

		invitorID := vars["userid"]
		email := vars["invited_address"]

		if invitorID == "" || email == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if !a.isAuthorizedUser(token, invitorID) {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		invite := &models.Confirmation{
			Email:     email,
			CreatorId: invitorID,
			Creator:   models.Creator{},
			Type:      models.TypeCareteamInvite,
		}

		if conf, err := a.findExistingConfirmation(req.Context(), invite, res); err != nil {
			log.Printf("CancelInvite: finding [%s]", err.Error())
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
		} else if conf != nil {
			//cancel the invite
			conf.UpdateStatus(models.StatusCanceled)

			if a.addOrUpdateConfirmation(req.Context(), conf, res) {
				a.logAudit(req, "cancelled invite")
				res.WriteHeader(http.StatusOK)
				return
			}
		}
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusNotFound, statusInviteNotFoundMessage)}
		log.Printf("CancelInvite: [%s]", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
		return
	}
	return
}

// @Summary Dismiss an invite
// @Description Invitee can dismiss an invite
// @ID hydrophone-api-dismissInvite
// @Accept  json
// @Produce  json
// @Param userid path string true "invitor id"
// @Param invitedby path string true "invited id"
// @Param invitation body models.Confirmation true "invitation details"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "inviteeid, invitorid or/and the payload is missing or malformed"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provided sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 404 {object} status.Status "invitation not found"
// @Failure 500 {object} status.Status "Error (internal) while processing the data"
// @Router /dismiss/invite/{userid}/{invitedby} [put]
// @security TidepoolAuth
func (a *Api) DismissInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {

		inviteeID := vars["userid"]
		invitorID := vars["invitedby"]

		if inviteeID == "" || invitorID == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		// Non-server tokens only legit when for same userid
		if !token.IsServer && inviteeID != token.UserId {
			log.Printf("DismissInvite %s ", STATUS_UNAUTHORIZED)
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusUnauthorized, STATUS_UNAUTHORIZED)}
			a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
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

		if conf, err := a.findExistingConfirmation(req.Context(), dismiss, res); err != nil {
			log.Printf("DismissInvite: finding [%s]", err.Error())
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
			return
		} else if conf != nil {

			conf.UpdateStatus(models.StatusDeclined)

			if a.addOrUpdateConfirmation(req.Context(), conf, res) {
				a.logAudit(req, "dismissinvite")
				res.WriteHeader(http.StatusOK)
				return
			}
		}
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusNotFound, statusInviteNotFoundMessage)}
		log.Printf("DismissInvite: [%s]", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
		return
	}
	return
}

// @Summary Cancel an invite
// @Description Admin can cancel a team invite sent to an HCP. A member can cancel an invite sent to a patient. A patient can cancel an invite sent to a caregiver
// @ID hydrophone-api-cancelAnyInvite
// @Accept  json
// @Produce  json
// @Param payload body models.Confirmation true "invitation details"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "payload is missing or malformed"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provide sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 404 {object} status.Status "invitation not found"
// @Failure 500 {object} status.Status "Error (internal) while processing the data"
// @Router /cancel/invite [post]
// @security TidepoolAuth
func (a *Api) CancelAnyInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	token := a.token(res, req)
	if token == nil {
		return
	}
	tokenValue := getSessionToken(req)

	cancel := &models.Confirmation{}
	if err := json.NewDecoder(req.Body).Decode(cancel); err != nil {
		log.Printf("CancelInvite: error decoding invite to dismiss [%v]", err)
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
		return
	}

	// mandatory fields from the request body
	if cancel.Key == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	if (cancel.Team == nil || cancel.Team.ID == "") && cancel.Email == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	cancel.Status = models.StatusPending
	if conf, err := a.findExistingConfirmation(req.Context(), cancel, res); err != nil {
		log.Printf("CancelInvite: finding [%s]", err.Error())
		a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
		return
	} else if conf != nil {

		var err error
		switch conf.Type {
		case models.TypeMedicalTeamPatientInvite:
			err = a.perms.RemovePatient(tokenValue, conf.Team.ID, conf.UserId)
		case models.TypeMedicalTeamInvite:
			if requestorIsAdmin, _, err := a.getTeamForUser(nil, tokenValue, cancel.Team.ID, token.UserId, res); err != nil {
				statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_UPDATING_TEAM)}
				a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
				return
			} else if !requestorIsAdmin {
				res.WriteHeader(http.StatusUnauthorized)
				return
			}
			if conf.UserId != "" {
				err = a.perms.RemoveTeamMember(tokenValue, conf.Team.ID, conf.UserId)
			}
		case models.TypeCareteamInvite:
			//verify the request comes from the creator
			if !a.isAuthorizedUser(token, conf.CreatorId) {
				a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
				return
			}
		default:
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if err != nil {
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_UPDATING_TEAM)}
			a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
			return
		}

		conf.UpdateStatus(models.StatusDeclined)

		if a.addOrUpdateConfirmation(req.Context(), conf, res) {
			log.Printf("cancel invite [%s]", cancel.Key)
			a.logAudit(req, "cancelInvite ")
			res.WriteHeader(http.StatusOK)
			return
		}
	}
	statusErr := &status.StatusError{Status: status.NewStatus(http.StatusNotFound, statusInviteNotFoundMessage)}
	log.Printf("CancelInvite: [%s]", statusErr.Error())
	a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
}

// @Summary Cancel all invites
// @Description Server token can cancel all team invites for a given user
// @ID hydrophone-api-cancelAllInvites
// @Accept  json
// @Produce  json
// @Param email path string true "invitee email address"
// @Success 200 {string} string "OK"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provide sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 500 {object} status.Status "Error (internal) while processing the data"
// @Router /cancel/all/{email} [post]
// @security TidepoolAuth
func (a *Api) CancelAllInvites(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	token := a.token(res, req)
	inviteeEmail := vars["email"]
	if !token.IsServer {
		a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
		return
	}

	invites, _ := a.Store.FindConfirmations(
		req.Context(),
		&models.Confirmation{CreatorId: "", Email: inviteeEmail},
		[]models.Status{models.StatusPending},
		[]models.Type{},
	)
	if len(invites) == 0 {
		return
	}

	for _, inv := range invites {
		inv.UpdateStatus(models.StatusCanceled)
		if !a.addOrUpdateConfirmation(req.Context(), inv, res) {
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION)}
			log.Printf("CancelAllInvite failed: [%s]", statusErr.Error())
			a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
			return
		}
	}
	log.Printf("cancel invites for [%s]", inviteeEmail)
	res.WriteHeader(http.StatusOK)
}

// @Summary Send an invite to join a patient's team
// @Description  Send a invite to new or existing users to join the patient's team
// @ID hydrophone-api-SendInvite
// @Accept  json
// @Produce  json
// @Param userid path string true "invitor user id"
// @Param payload body InviteBody true "invitation details"
// @Success 200 {object} models.Confirmation "invite details"
// @Failure 400 {object} status.Status "userId was not provided or the payload is missing/malformed"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provided sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 409 {object} status.Status "user already has a pending or declined invite OR user is already part of the team"
// @Failure 422 {object} status.Status "Error when sending the email (probably caused by the mailling service"
// @Failure 500 {object} status.Status "Internal error while processing the invite, detailled error returned in the body"
// @Router /send/invite/{userid} [post]
// @security TidepoolAuth
func (a *Api) SendInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	// By default, the invitee language will be "en" for Englih (as we don't know which language suits him)
	// In case the invitee is a known user, the language will be overriden in a later step
	var inviteeLanguage = GetUserChosenLanguage(req)
	if token := a.token(res, req); token != nil {

		invitorID := vars["userid"]

		if invitorID == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if !a.isAuthorizedUser(token, invitorID) {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		defer req.Body.Close()
		var ib = &InviteBody{}
		if err := json.NewDecoder(req.Body).Decode(ib); err != nil {
			log.Printf("SendInvite: error decoding invite to detail %v\n", err)
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
			return
		}

		if ib.Email == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if existingInvite, invitedUsr := a.checkForDuplicateInvite(req.Context(), ib.Email, invitorID, getSessionToken(req), res); existingInvite == true {
			log.Printf("SendInvite: invited [%s] user already has or had an invite", ib.Email)
			return
		} else {

			if invitedUsr != nil && invitedUsr.HasRole("patient") {
				statusErr := &status.StatusError{Status: status.NewStatus(http.StatusMethodNotAllowed, STATUS_PATIENT_NOT_CAREGIVER)}
				a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
				return
			}

			//None exist so lets create the invite
			invite, _ := models.NewConfirmation(models.TypeCareteamInvite, models.TemplateNameCareteamInvite, invitorID)

			// if the invitee is already a Tidepool user, we can use his preferences
			invite.Email = ib.Email
			if invitedUsr != nil {
				invite.UserId = invitedUsr.UserID

				// let's get the invitee user preferences
				inviteeLanguage = a.getUserLanguage(invite.UserId, req, res)
			}

			if a.addOrUpdateConfirmation(req.Context(), invite, res) {
				a.logAudit(req, "invite created")

				if err := a.addProfile(req.Context(), invite); err != nil {
					log.Println("SendInvite: ", err.Error())
				} else {

					fullName := invite.Creator.Profile.FullName

					var webPath = "signup"

					// if invitee is already a user (ie already has an account), he won't go to signup but login instead
					if invite.UserId != "" {
						webPath = "login"
					}

					emailContent := map[string]string{
						"PatientName": fullName,
						"Email":       invite.Email,
						"WebPath":     webPath,
						"Duration":    invite.GetReadableDuration(),
					}

					if a.createAndSendNotification(req, invite, emailContent, inviteeLanguage) {
						a.logAudit(req, "invite sent")
					} else {
						a.logAudit(req, "invite failed to be sent")
						log.Print("Something happened generating an invite email")
						res.WriteHeader(http.StatusUnprocessableEntity)
						return
					}
				}

				a.sendModelAsResWithStatus(res, invite, http.StatusOK)
				return
			}
		}

	}
	return
}
