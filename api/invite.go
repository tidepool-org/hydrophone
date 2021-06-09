package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/mdblp/crew/store"
	"github.com/mdblp/hydrophone/models"
	"github.com/mdblp/shoreline/schema"
	"github.com/tidepool-org/go-common/clients/status"
)

const (
	//Status message we return from the service
	statusExistingInviteMessage  = "There is already an existing invite"
	statusExistingMemberMessage  = "The user is already an existing member"
	statusInviteNotFoundMessage  = "No matching invite was found"
	statusInviteCanceledMessage  = "Invite has been canceled"
	statusInviteNotActiveMessage = "Invite already canceled"
	statusForbiddenMessage       = "Forbidden to perform requested operation"
)

type (
	//Invite details for generating a new invite
	inviteBody struct {
		Email  string `json:"email"`
		User   string `json:"user"`
		TeamID string `json:"teamId"`
		Role   string `json:"role"`
	}
)

//Checks do they have an existing invite or are they already a team member
//Or are they an existing user and already in the group?
func (a *Api) checkForDuplicateInvite(ctx context.Context, inviteeEmail, invitorID, token string, res http.ResponseWriter) (bool, *schema.UserData) {

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

//Checks do they have an existing invite or are they already a team member
//Or are they an existing user and already in the group?
func (a *Api) checkForDuplicateTeamInvite(ctx context.Context, inviteeEmail, invitorID, token string, team store.Team, invite models.Type, res http.ResponseWriter) (bool, *schema.UserData) {

	//already has invite from this user?
	invites, _ := a.Store.FindConfirmations(
		ctx,
		&models.Confirmation{
			Email: inviteeEmail,
			Team: &models.Team{
				ID: team.ID,
			},
			Type: invite},
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
	// call the teams service to check if the user is already a member
	if invitedUsr != nil && invite == models.TypeMedicalTeamInvite {
		if isMember := a.isTeamMember(invitedUsr.UserID, team, false); isMember {
			log.Printf("checkForDuplicateTeamInvite: invited [%s] user is already a member of [%s]", inviteeEmail, team.Name)
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusConflict, statusExistingMemberMessage)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
			return true, invitedUsr
		}
		return false, invitedUsr
	}
	if invitedUsr != nil && invite == models.TypeMedicalTeamPatientInvite {
		members, err := a.perms.GetTeamPatients(token, team.ID)
		if err != nil {
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_FINDING_TEAM)}
			a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
			return true, invitedUsr
		}
		for i := 0; i < len(members); i++ {
			if members[i].UserID == invitedUsr.UserID && members[i].InvitationStatus != "rejected" {
				statusErr := &status.StatusError{Status: status.NewStatus(http.StatusConflict, statusExistingMemberMessage)}
				a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
				return true, invitedUsr
			}
		}
		return false, invitedUsr
	}
	return false, nil
}

func (a *Api) getUserPreferences(userid string, res http.ResponseWriter) *models.Preferences {
	// let's get the invitee user preferences
	inviteePreferences := &models.Preferences{}
	if err := a.seagull.GetCollection(userid, "preferences", a.sl.TokenProvide(), inviteePreferences); err != nil {
		a.sendError(
			res,
			http.StatusInternalServerError,
			STATUS_ERR_FINDING_USR,
			"send invitation: error getting invitee user preferences: ",
			err.Error())
	}
	return inviteePreferences
}

func (a *Api) getUserLanguage(userid string, res http.ResponseWriter) string {
	// let's get the invitee user preferences
	language := "en"
	inviteePreferences := a.getUserPreferences(userid, res)
	// does the invitee have a preferred language?
	if inviteePreferences.DisplayLanguage != "" {
		language = inviteePreferences.DisplayLanguage
	}
	return language
}

func (a *Api) isTeamAdmin(userid string, team store.Team) bool {
	for j := 0; j < len(team.Members); j++ {
		if team.Members[j].UserID == userid {
			if team.Members[j].Role == "admin" {
				return true
			}
		}
	}
	return false
}

// @Summary Get list of received invitations for logged-in user
// @Description  Get list of received invitations that have been sent to this user but not yet acted upon.
// @ID hydrophone-api-GetReceivedInvitations
// @Accept  json
// @Produce  json
// @Param userid path string true "user id"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "usereid was not provided"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provided sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 404 {object} status.Status "no invitations found for this user"
// @Failure 500 {object} status.Status "Error while extracting the data"
// @Router /invitations/{userid} [get]
// @security TidepoolAuth
func (a *Api) GetReceivedInvitations(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		tokenValue := req.Header.Get(TP_SESSION_TOKEN)
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

		invitedUsr := a.findExistingUser(inviteeID, req.Header.Get(TP_SESSION_TOKEN))

		types := []models.Type{}
		// Show invites relevant for the type of user
		if invitedUsr.HasRole("patient") {
			types = append(types, models.TypeMedicalTeamPatientInvite)
		}
		if invitedUsr.HasRole("caregiver") {
			types = append(types, models.TypeCareteamInvite, models.TypeMedicalTeamInvite)
		}
		if invitedUsr.HasRole("hcp") {
			types = append(types, models.TypeMedicalTeamInvite,
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

		if invites := a.checkFoundConfirmations(tokenValue, res, found, err); invites != nil {
			a.ensureIdSet(req.Context(), inviteeID, invites)
			log.Printf("GetReceivedInvitations: found and have checked [%d] invites ", len(invites))
			a.logAudit(req, "get received invites")
			a.sendModelAsResWithStatus(res, invites, http.StatusOK)
		}
	}
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

	tokenValue := req.Header.Get(TP_SESSION_TOKEN)
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
		&models.Confirmation{CreatorId: invitorID, Type: models.TypeCareteamInvite},
		[]models.Status{models.StatusPending},
		[]models.Type{models.TypeCareteamInvite, models.TypeMedicalTeamInvite, models.TypeMedicalTeamPatientInvite},
	)
	if invitations := a.checkFoundConfirmations(tokenValue, res, found, err); invitations != nil {
		a.logAudit(req, "get sent invites")
		a.sendModelAsResWithStatus(res, invitations, http.StatusOK)
		return
	}
}

//Accept the given invite
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

//Accept the given invite
//
// http.StatusOK when accepted
// http.StatusBadRequest when the incoming data is incomplete or incorrect
// http.StatusForbidden when mismatch of user ID's, type or status
// @Summary Accept the given invite
// @Description  This would be PUT by the web page at the link in the invite email. No authentication is required.
// @ID hydrophone-api-acceptTeamNotifs
// @Accept  json
// @Produce  json
// @Param invitation body models.Confirmation true "invitation details"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "the payload is missing or malformed: key is not provided"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provided sufficient privileges"
// @Failure 403 {object} status.Status "Operation is forbiden. The invitation cannot be accepted for this given user"
// @Failure 404 {object} status.Status "invitation not found"
// @Failure 500 {object} status.Status "Error (internal) while processing the data"
// @Router /accept/team/invite [put]
// @security TidepoolAuth
func (a *Api) AcceptTeamNotifs(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	token := a.token(res, req)
	if token == nil {
		return
	}
	if token.Role != "hcp" && token.Role != "patient" {
		a.sendModelAsResWithStatus(
			res,
			&status.StatusError{Status: status.NewStatus(http.StatusForbidden, "Caregivers cannot accept a team invitation")},
			http.StatusForbidden,
		)
		return
	}

	inviteeID := token.UserId

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
		ValidateType([]models.Type{
			models.TypeMedicalTeamDoAdmin,
			models.TypeMedicalTeamRemove,
			models.TypeMedicalTeamInvite,
			models.TypeMedicalTeamPatientInvite,
		}, &validationErrors).
		ValidateUserID(inviteeID, &validationErrors)

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

	switch conf.Type {
	case models.TypeMedicalTeamPatientInvite, models.TypeMedicalTeamInvite:
		a.acceptTeamInvite(res, req, conf)
	default:
		a.acceptAnyInvite(res, req, conf)
	}

	log.Printf("AcceptInvite: permissions were set for [%v] after an invite was accepted", inviteeID)

}

func (a *Api) acceptAnyInvite(res http.ResponseWriter, req *http.Request, conf *models.Confirmation) {
	conf.UpdateStatus(models.StatusCompleted)
	if !a.addOrUpdateConfirmation(req.Context(), conf, res) {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION)}
		log.Println("AcceptAnyInvite ", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusInternalServerError)
		return
	}
	a.logAudit(req, "acceptanyinvite")
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(STATUS_OK))
}

func (a *Api) acceptTeamInvite(res http.ResponseWriter, req *http.Request, conf *models.Confirmation) {

	var member = store.Member{
		UserID:           conf.UserId,
		TeamID:           conf.Team.ID,
		InvitationStatus: "accepted",
		Role:             conf.Role,
	}
	// are we updating a team member or a patient
	var err error
	if conf.Role != "patient" {
		_, err = a.perms.AddTeamMember(a.sl.TokenProvide(), member)
	} else {
		_, err = a.perms.AddOrUpdatePatient(req.Header.Get(TP_SESSION_TOKEN), member)
	}
	if err != nil {
		log.Printf("AcceptInvite error setting permissions [%v]\n", err)
		a.sendModelAsResWithStatus(
			res,
			&status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_DECODING_CONFIRMATION)},
			http.StatusInternalServerError,
		)
		return
	}

	log.Printf("AcceptInvite: permissions were set for [%v -> %v] after an invite was accepted", conf.Team.ID, conf.UserId)
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

// @Summary Dismiss a team invite
// @Description Invitee or Admin can dismiss a team invite. A patient can dismiss a care team invite.
// @ID hydrophone-api-dismissTeamInvite
// @Accept  json
// @Produce  json
// @Param teamid path string true "Team ID"
// @Success 200 {string} string "OK"
// @NotModified 304 {string} "not modified"
// @Failure 400 {object} status.Status "inviteeid or/and the payload is missing or malformed"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provided sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 404 {object} status.Status "invitation not found"
// @Failure 500 {object} status.Status "Error (internal) while processing the data"
// @Router /dismiss/team/invite/{teamid} [put]
// @security TidepoolAuth
func (a *Api) DismissTeamInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	token := a.token(res, req)
	if token == nil {
		return
	}

	teamID := vars["teamid"]
	// either the token is the inviteeID or the admin ID
	// let's find out what type of user it is later on
	userID := token.UserId

	if teamID == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	dismiss := &models.Confirmation{}
	if err := json.NewDecoder(req.Body).Decode(dismiss); err != nil {
		log.Printf("DismissInvite: error decoding invite to dismiss [%v]", err)
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
		return
	}

	// key of the request
	if dismiss.Key == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	tokenValue := req.Header.Get(TP_SESSION_TOKEN)
	// by default you can just act on your records
	dismiss.UserId = userID
	dismiss.Team = &models.Team{ID: teamID}

	if isAdmin, _, err := a.getTeamForUser(tokenValue, teamID, token.UserId, res); isAdmin && err == nil {
		// as team admin you can act on behalf of members
		// for any invitation for the given team
		dismiss.UserId = ""
	}

	if conf, err := a.findExistingConfirmation(req.Context(), dismiss, res); err != nil {
		log.Printf("DismissInvite: finding [%s]", err.Error())
		a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
		return
	} else if conf != nil {

		if conf.Status != models.StatusDeclined && conf.Status != models.StatusCanceled {

			var member = store.Member{
				UserID:           conf.UserId,
				TeamID:           teamID,
				InvitationStatus: "rejected",
			}

			var err error
			switch conf.Type {
			case models.TypeMedicalTeamPatientInvite:
				_, err = a.perms.AddOrUpdatePatient(tokenValue, member)
			default:
				_, err = a.perms.UpdateTeamMember(tokenValue, member)
			}
			if err != nil {
				statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_UPDATING_TEAM)}
				a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
				return
			}

			conf.UpdateStatus(models.StatusDeclined)

			if a.addOrUpdateConfirmation(req.Context(), conf, res) {
				log.Printf("dismiss invite [%s] for [%s]", dismiss.Key, dismiss.Team.ID)
				a.logAudit(req, "dismissinvite ")
				res.WriteHeader(http.StatusOK)
				return
			}
		}
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusNotModified, statusInviteNotActiveMessage)}
		log.Printf("DismissInvite: [%s]", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}
	statusErr := &status.StatusError{Status: status.NewStatus(http.StatusNotFound, statusInviteNotFoundMessage)}
	log.Printf("DismissInvite: [%s]", statusErr.Error())
	a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
}

// @Summary Cancel an invite
// @Description Admin can cancel a team invite sent to an HCP. A member can cancel an invite sent to a patient. A patient can cancel an invite sent to a caregiver
// @ID hydrophone-api-cancelAnyInvite
// @Accept  json
// @Produce  json
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
	tokenValue := req.Header.Get(TP_SESSION_TOKEN)

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
			if requestorIsAdmin, _, err := a.getTeamForUser(tokenValue, cancel.Team.ID, token.UserId, res); err != nil {
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

// @Summary Send a invite to join a patient's team
// @Description  Send a invite to new or existing users to join the patient's team
// @ID hydrophone-api-SendInvite
// @Accept  json
// @Produce  json
// @Param userid path string true "invitor user id"
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
		var ib = &inviteBody{}
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

		if existingInvite, invitedUsr := a.checkForDuplicateInvite(req.Context(), ib.Email, invitorID, req.Header.Get(TP_SESSION_TOKEN), res); existingInvite == true {
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
				inviteePreferences := &models.Preferences{}
				if err := a.seagull.GetCollection(invite.UserId, "preferences", a.sl.TokenProvide(), inviteePreferences); err != nil {
					a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, "send invitation: error getting invitee user preferences: ", err.Error())
					return
				}
				// does the invitee have a preferred language?
				if inviteePreferences.DisplayLanguage != "" {
					inviteeLanguage = inviteePreferences.DisplayLanguage
				}
			}

			if a.addOrUpdateConfirmation(req.Context(), invite, res) {
				a.logAudit(req, "invite created")

				if err := a.addProfile(invite); err != nil {
					log.Println("SendInvite: ", err.Error())
				} else {

					fullName := invite.Creator.Profile.FullName

					if invite.Creator.Profile.Patient.IsOtherPerson {
						fullName = invite.Creator.Profile.Patient.FullName
					}

					var webPath = "signup"

					// if invitee is already a user (ie already has an account), he won't go to signup but login instead
					if invite.UserId != "" {
						webPath = "login"
					}

					emailContent := map[string]interface{}{
						"PatientName": fullName,
						"Email":       invite.Email,
						"WebPath":     webPath,
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

// @Summary Send invitation to a hcp or patient for joining a medical team
// @Description  create a notification for the invitee and send him an email with the invitation. The patient account has to exist otherwise the invitation is rejected.
// @ID hydrophone-api-SendTeamInvite
// @Accept  json
// @Produce  json
// @Success 200 {object} models.Confirmation "invite details"
// @Failure 400 {object} status.Status "userId, teamId and isAdmin were not provided or the payload is missing/malformed"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provide sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 405 {object} status.Status "HCP cannot invite a patient to care team"
// @Failure 409 {object} status.Status "user already has a pending or declined invite OR user is already part of the team"
// @Failure 422 {object} status.Status "Error when sending the email (probably caused by the mailling service"
// @Failure 500 {object} status.Status "Internal error while processing the invite, detailled error returned in the body"
// @Router /confirm/send/team/invite [post]
// @security TidepoolAuth
func (a *Api) SendTeamInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	// By default, the invitee language will be "en" for Englih (as we don't know which language suits him)
	// In case the invitee is a known user, the language will be overriden in a later step
	var inviteeLanguage = GetUserChosenLanguage(req)
	tokenValue := req.Header.Get(TP_SESSION_TOKEN)
	token := a.token(res, req)
	if token == nil {
		return
	}

	invitorID := token.UserId
	if invitorID == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	defer req.Body.Close()
	var ib = &inviteBody{}
	if err := json.NewDecoder(req.Body).Decode(ib); err != nil {
		log.Printf("SendInvite: error decoding invite to detail %v\n", err)
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_INVITE)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}

	if ib.Email == "" || ib.TeamID == "" {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_MISSING_DATA_INVITE)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}
	managePatients := false
	inviteType := models.TypeMedicalTeamInvite
	switch strings.ToLower(ib.Role) {
	case "admin":
		ib.Role = "admin"
	case "patient":
		ib.Role = "patient"
		managePatients = true
		inviteType = models.TypeMedicalTeamPatientInvite
	default:
		ib.Role = "member"
	}

	auth, team, _ := a.getTeamForUser(tokenValue, ib.TeamID, token.UserId, res)

	// only for team management
	if !auth && !managePatients {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_NOT_ADMIN)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}

	// check duplicate invite and if user is already a member
	if existingInvite, invitedUsr := a.checkForDuplicateTeamInvite(req.Context(), ib.Email, invitorID, tokenValue, team, inviteType, res); existingInvite {
		return
	} else {
		// lets create the invite depending o type of invited member
		var invite *models.Confirmation
		var member = store.Member{
			TeamID:           ib.TeamID,
			Role:             ib.Role,
			InvitationStatus: "pending",
		}
		if managePatients {
			if statusErr := a.invitePatient(invitedUsr, member, tokenValue); statusErr != nil {
				a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
				return
			}
			invite, _ = models.NewConfirmation(
				models.TypeMedicalTeamPatientInvite,
				models.TemplateNameMedicalteamPatientInvite,
				invitorID)
		} else {
			if statusErr := a.inviteHcp(invitedUsr, member, tokenValue); statusErr != nil {
				a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
				return
			}
			invite, _ = models.NewConfirmation(
				models.TypeMedicalTeamInvite,
				models.TemplateNameMedicalteamInvite,
				invitorID)
		}
		// complete invite
		invite.Team = &models.Team{ID: ib.TeamID}
		invite.Email = ib.Email
		invite.Role = ib.Role
		if invitedUsr != nil {
			invite.UserId = invitedUsr.UserID
			inviteeLanguage = a.getUserLanguage(invite.UserId, res)
		}
		if a.addOrUpdateConfirmation(req.Context(), invite, res) {
			a.logAudit(req, "invite created")

			if err := a.addProfile(invite); err != nil {
				log.Println("SendInvite: ", err.Error())
			} else {
				var webPath = ""
				if !managePatients && invite.UserId == "" {
					webPath = "signup"
				}

				emailContent := map[string]interface{}{
					"MedicalteamName":          team.Name,
					"MedicalteamAddress":       team.Address,
					"MedicalteamPhone":         team.Phone,
					"MedicalteamIentification": team.Code,
					"CreatorName":              invite.Creator.Profile.FullName,
					"Email":                    invite.Email,
					"WebPath":                  webPath,
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

func (a *Api) invitePatient(invitedUsr *schema.UserData, member store.Member, token string) *status.StatusError {
	if invitedUsr == nil {
		// we return an error as the invitedUser does not exist yet
		return &status.StatusError{Status: status.NewStatus(http.StatusForbidden, STATUS_ERR_FINDING_USER)}
	}
	// non-patient cannot be invited as a patient of a care team
	if !invitedUsr.HasRole("patient") {
		return &status.StatusError{Status: status.NewStatus(http.StatusMethodNotAllowed, STATUS_MEMBER_NOT_AUTH)}
	}
	member.UserID = invitedUsr.UserID
	if _, err := a.perms.AddOrUpdatePatient(token, member); err != nil {
		return &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_UPDATING_TEAM)}
	} else {
		log.Printf("Add patient %s in Team %s", invitedUsr.UserID, member.TeamID)
		return nil
	}
}

func (a *Api) inviteHcp(invitedUsr *schema.UserData, member store.Member, token string) *status.StatusError {
	if invitedUsr == nil {
		return nil
	}
	// patient cannot be invited as a member
	if invitedUsr.HasRole("patient") {
		return &status.StatusError{Status: status.NewStatus(http.StatusMethodNotAllowed, STATUS_PATIENT_NOT_AUTH)}
	}
	member.UserID = invitedUsr.UserID
	if _, err := a.perms.AddTeamMember(token, member); err != nil {
		return &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_UPDATING_TEAM)}
	} else {
		log.Printf("Add member %s in Team %s", invitedUsr.UserID, member.TeamID)
		return nil
	}
}

// @Summary Send notification to an hcp that becomes admin
// @Description  Send an email and a notification to the new admin user. The role change is done but notification is pushed for information (notification status is set to pending). Removing the admin role is managed as an exception. It does not trigger any notification or email.
// @ID hydrophone-api-UpdateTeamRole
// @Accept  json
// @Produce  json
// @Param userid path string true "user id"
// @Success 200 {object} models.Confirmation "invite details"
// @Failure 400 {object} status.Status "userId, teamId and isAdmin were not provided or the payload is missing/malformed"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provide sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 409 {object} status.Status "No notification and email sent; User is already an admin"
// @Failure 422 {object} status.Status "Error when sending the email (probably caused by the mailling service"
// @Failure 500 {object} status.Status "Internal error while processing the invite, detailled error returned in the body"
// @Router /confirm/send/team/role/{userid} [put]
// @security TidepoolAuth
func (a *Api) UpdateTeamRole(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	// By default, the invitee language will be "en" for Englih (as we don't know which language suits him)
	// In case the invitee is a known user, the language will be overriden in a later step
	var inviteeLanguage = GetUserChosenLanguage(req)
	tokenValue := req.Header.Get(TP_SESSION_TOKEN)
	token := a.token(res, req)
	if token == nil {
		return
	}
	inviteeID := vars["userid"]
	if inviteeID == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	invitorID := token.UserId
	if invitorID == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	defer req.Body.Close()
	var ib = &inviteBody{}
	if err := json.NewDecoder(req.Body).Decode(ib); err != nil {
		log.Printf("UpdateInvite: error decoding invite to detail %v\n", err)
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_INVITE)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}

	if ib.TeamID == "" {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_MISSING_DATA_INVITE)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}
	if strings.ToLower(ib.Role) == "admin" {
		ib.Role = "admin"
	}

	_, team, err := a.getTeamForUser(tokenValue, ib.TeamID, token.UserId, res)
	if err != nil {
		return
	}
	if isMember := a.isTeamMember(inviteeID, team, false); !isMember {
		// the invitee is not an accepted member of the team
		log.Printf("UpdateInvite: %s is not a member of %s", inviteeID, team.ID)
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_NOT_ADMIN)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}

	if admin := a.isTeamAdmin(inviteeID, team); admin == (ib.Role == "admin") {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusConflict, STATUS_ROLE_ALRDY_ASSIGNED)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}
	var member = store.Member{
		UserID: inviteeID,
		TeamID: ib.TeamID,
		Role:   ib.Role,
	}
	if _, err := a.perms.UpdateTeamMember(tokenValue, member); err != nil {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_UPDATING_TEAM)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}
	// Send the notification and email when adding admin role
	if ib.Role == "admin" {
		invite, _ := models.NewConfirmation(
			models.TypeMedicalTeamDoAdmin,
			models.TemplateNameMedicalteamDoAdmin,
			invitorID)

		// if the invitee is already a user, we can use his preferences
		invite.Team.ID = ib.TeamID
		invite.Email = ib.Email
		invite.Role = ib.Role
		invite.Status = models.StatusPending
		invite.UserId = inviteeID
		// does the invitee have a preferred language?
		inviteeLanguage = a.getUserLanguage(invite.UserId, res)
		if a.addOrUpdateConfirmation(req.Context(), invite, res) {
			a.logAudit(req, "invite created")

			if err := a.addProfile(invite); err != nil {
				log.Println("SendInvite: ", err.Error())
			} else {

				emailContent := map[string]interface{}{
					"MedicalteamName": team.Name,
					"Email":           invite.Email,
					"Language":        inviteeLanguage,
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

// @Summary Delete hcp from a medical team
// @Description create a notification for the hcp and send him an email to inform him he has been removed from the medical team
// @ID hydrophone-api-DeleteTeamMember
// @Accept  json
// @Produce  json
// @Param userid path string true "user id"
// @Success 200 {object} models.Confirmation "delete member"
// @Failure 400 {object} status.Status "userId, teamId and isAdmin were not provided or the payload is missing/malformed"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provide sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 409 {object} status.Status "No notification and email sent; User is not a member"
// @Failure 422 {object} status.Status "Error when sending the email (probably caused by the mailling service"
// @Failure 500 {object} status.Status "Internal error while processing the invite, detailled error returned in the body"
// @Router /confirm/send/team/leave/{userid} [delete]
// @security TidepoolAuth
func (a *Api) DeleteTeamMember(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	// By default, the invitee language will be "en" for Englih (as we don't know which language suits him)
	// In case the invitee is a known user, the language will be overriden in a later step
	var inviteeLanguage = GetUserChosenLanguage(req)
	tokenValue := req.Header.Get(TP_SESSION_TOKEN)
	token := a.token(res, req)
	if token == nil {
		return
	}

	inviteeID := vars["userid"]
	if inviteeID == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	invitorID := token.UserId
	if invitorID == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	defer req.Body.Close()
	var ib = &inviteBody{}
	if err := json.NewDecoder(req.Body).Decode(ib); err != nil {
		log.Printf("UpdateInvite: error decoding invite to detail %v\n", err)
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_INVITE)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}

	if ib.Email == "" || ib.TeamID == "" {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_MISSING_DATA_INVITE)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}

	_, team, err := a.getTeamForUser(tokenValue, ib.TeamID, token.UserId, res)
	if err != nil {
		return
	}

	if isMember := a.isTeamMember(inviteeID, team, true); !isMember {
		// the invitee is not a member of the team
		log.Printf("UpdateInvite: %s is not a member of %s", inviteeID, team.ID)
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_NOT_MEMBER)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}

	if admin := a.isTeamAdmin(invitorID, team); !admin {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusConflict, STATUS_UNAUTHORIZED)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}

	invite, _ := models.NewConfirmation(
		models.TypeMedicalTeamRemove,
		models.TemplateNameMedicalteamRemove,
		invitorID)

	// let's use the user preferences
	invite.Team.ID = ib.TeamID
	invite.Email = ib.Email
	invite.Role = ib.Role
	invite.UserId = inviteeID
	// does the invitee have a preferred language?
	inviteeLanguage = a.getUserLanguage(invite.UserId, res)

	if err := a.perms.RemoveTeamMember(tokenValue, ib.TeamID, invite.UserId); err != nil {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_UPDATING_TEAM)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}

	if a.addOrUpdateConfirmation(req.Context(), invite, res) {
		a.logAudit(req, "invite created")

		if err := a.addProfile(invite); err != nil {
			log.Println("SendInvite: ", err.Error())
		} else {

			emailContent := map[string]interface{}{
				"MedicalteamName": team.Name,
				"Email":           invite.Email,
				"Language":        inviteeLanguage,
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

	return
}

// userId is member of a Team
// Settings the all parameter to true will return all the members while it will only return the accepted members if the parameter is set false
func (a *Api) isTeamMember(userID string, team store.Team, all bool) bool {
	for i := 0; i < len(team.Members); i++ {
		if team.Members[i].UserID == userID && (team.Members[i].InvitationStatus == "accepted" || all) {
			return true
		}
	}
	return false
}

//
// return true is the user userID is admin of the Team identified by teamID
// it returns the Team object corresponding to the team
// if any error occurs during the search, it returns an error with the
// related code
func (a *Api) getTeamForUser(token, teamID, userID string, res http.ResponseWriter) (bool, store.Team, error) {
	var auth = false
	team, err := a.perms.GetTeam(token, teamID)
	if err != nil {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_FINDING_TEAM)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return auth, store.Team{}, err
	}
	auth = a.isTeamAdmin(userID, *team)
	return auth, *team, nil
}
