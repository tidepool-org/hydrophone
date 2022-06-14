package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mdblp/crew/store"
	"github.com/mdblp/go-common/clients/status"
	"github.com/mdblp/hydrophone/models"
	"github.com/mdblp/shoreline/schema"
)

func formatAddress(addr store.Address) string {
	if addr.Line2 != "" {
		return fmt.Sprintf("%s %s, %s %s, %s", addr.Line1, addr.Line2, addr.Zip, addr.City, addr.Country)
	} else {
		return fmt.Sprintf("%s, %s %s, %s", addr.Line1, addr.Zip, addr.City, addr.Country)
	}
}

//Checks do they have an existing invite or are they already a team member
//Or are they an existing user and already in the group?
func (a *Api) checkForDuplicateTeamInvite(ctx context.Context, inviteeEmail, invitorID, token string, team store.Team, invite models.Type, res http.ResponseWriter) (bool, *schema.UserData) {

	confirmation := &models.Confirmation{
		Email: inviteeEmail,
		Team: &models.Team{
			ID: team.ID,
		},
		Type: invite}

	//already has invite from this user?
	invites, _ := a.Store.FindConfirmations(
		ctx,
		confirmation,
		[]models.Status{models.StatusPending},
		[]models.Type{},
	)
	inviteeID := confirmation.Email
	if confirmation.Email == "" {
		inviteeID = confirmation.UserId
	}

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

	invitedUsr := a.findExistingUser(inviteeID, a.sl.TokenProvide())
	// call the teams service to check if the hcp user is already a member
	if invitedUsr != nil && invite == models.TypeMedicalTeamInvite {
		if isMember, _ := a.isTeamMember(invitedUsr.UserID, team, false); isMember {
			log.Printf("checkForDuplicateTeamInvite: invited [%s] user is already a member of [%s]", sanitize(inviteeID), team.Name)
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

// return the user and its status in the team, true it can be invited for monitor, otherwise it returns false
func (a *Api) checkForMonitoringTeamInviteById(ctx context.Context, inviteeID, invitorID, token string, team store.Team, res http.ResponseWriter) (bool, *schema.UserData) {
	confirmation := &models.Confirmation{
		UserId: inviteeID,
		Team: &models.Team{
			ID: team.ID,
		}}
	//already has invite from this user?
	invites, _ := a.Store.FindConfirmations(
		ctx,
		confirmation,
		[]models.Status{models.StatusPending},
		[]models.Type{models.TypeMedicalTeamMonitoringInvite},
	)
	if len(invites) > 0 {
		//rule is we cannot send if the invite is not yet expired
		if !invites[0].IsExpired() {
			log.Println(statusExistingInviteMessage)
			log.Println("last invite not yet expired")
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusConflict, statusExistingInviteMessage)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
			return false, nil
		}
	}
	invitedUsr := a.findExistingUser(inviteeID, a.sl.TokenProvide())
	// the invitedUser has to be a patient of the team
	if invitedUsr != nil {
		members, err := a.perms.GetTeamPatients(token, team.ID)
		if err != nil {
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_FINDING_TEAM)}
			a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
			return false, invitedUsr
		}
		for i := 0; i < len(members); i++ {
			if members[i].UserID == invitedUsr.UserID && members[i].InvitationStatus == "accepted" {
				return true, invitedUsr
			}
		}
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_PATIENT_NOT_MBR)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return false, invitedUsr
	}
	statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_FINDING_USER)}
	a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
	return false, nil
}

func (a *Api) isTeamAdmin(userid string, team store.Team) bool {
	for j := 0; j < len(team.Members); j++ {
		if team.Members[j].UserID == userid {
			if team.Members[j].Role == "admin" {
				return true
			}
			break
		}
	}
	return false
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
// @security Authorization Bearer token
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

// Accept the given invite
// http.StatusOK when accepted
// http.StatusBadRequest when the incoming data is incomplete or incorrect
// http.StatusForbidden when mismatch of user ID's, type or status
// @Summary Accept the given invite
// @Description  This would be PUT by the web page at the link in the invite email. No authentication is required.
// @ID hydrophone-api-AcceptMonitoringInvite
// @Accept  json
// @Produce  json
// @Param teamid path string true "Team ID"
// @Param userid path string true "User ID"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "the payload is missing or malformed"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provided sufficient privileges"
// @Failure 403 {object} status.Status "Operation is forbiden. The invitation cannot be accepted for this given user"
// @Failure 404 {object} status.Status "invitation not found"
// @Failure 409 {object} status.Status "invitation is expired"
// @Failure 500 {object} status.Status "Error (internal) while processing the data"
// @Router /accept/team/monitoring/{teamid}/{userid} [put]
// @security TidepoolAuth
func (a *Api) AcceptMonitoringInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	action := "AcceptMonitoringInvite"
	token := a.token(res, req)
	if token == nil {
		return
	}
	if token.Role != "patient" {
		a.sendModelAsResWithStatus(
			res,
			&status.StatusError{Status: status.NewStatus(http.StatusForbidden, "Only Patients can accept a team monitoring invitation")},
			http.StatusForbidden,
		)
		return
	}
	userid := vars["userid"]
	teamid := vars["teamid"]

	if userid != token.UserId {
		a.sendModelAsResWithStatus(
			res,
			&status.StatusError{Status: status.NewStatus(http.StatusForbidden, STATUS_UNAUTHORIZED)},
			http.StatusForbidden,
		)
		return
	}

	accept := &models.Confirmation{
		UserId: userid,
		Team:   &models.Team{ID: teamid},
		Type:   models.TypeMedicalTeamMonitoringInvite,
	}

	conf, err := a.findExistingConfirmation(req.Context(), accept, res)
	if err != nil {
		log.Printf("%s error while finding confirmation [%s]\n", action, err.Error())
		a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
		return
	}
	if conf == nil {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusNotFound, statusInviteNotFoundMessage)}
		log.Printf("%s: [%s] ", action, statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
		return
	}
	if conf.IsExpired() {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusConflict, statusExpiredMessage)}
		log.Printf("%s: [%s] ", action, statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
		return
	}

	validationErrors := []error{}

	conf.ValidateStatus(models.StatusPending, &validationErrors).
		ValidateType([]models.Type{
			models.TypeMedicalTeamMonitoringInvite,
		}, &validationErrors).
		ValidateUserID(userid, &validationErrors)

	if len(validationErrors) > 0 {
		for _, validationError := range validationErrors {
			log.Printf("%s forbidden as there was a expectation mismatch %s", action, validationError)
		}
		a.sendModelAsResWithStatus(
			res,
			&status.StatusError{Status: status.NewStatus(http.StatusForbidden, statusForbiddenMessage)},
			http.StatusForbidden,
		)
		return
	}

	monitoring := make(map[string]interface{})
	monitoring["acceptanceTimestamp"] = time.Now().UTC().Format(time.RFC3339)
	monitoring["isAccepted"] = true
	patientMonitoringConsent := make(map[string]interface{})
	patientMonitoringConsent["monitoring"] = monitoring

	profileUpdate := make(map[string]interface{})
	profileUpdate["patient"] = patientMonitoringConsent

	err = a.seagull.SetCollection(conf.UserId, "profile", a.sl.TokenProvide(), profileUpdate)
	if err != nil {
		log.Printf("%s error getting monitord patient [%v]\n", action, err)
		a.sendModelAsResWithStatus(
			res,
			&status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_MONITORED_PATIENT)},
			http.StatusInternalServerError,
		)
		return
	}
	patientMonitored, err := a.perms.GetPatientMonitoring(req.Context(), a.sl.TokenProvide(), conf.UserId, conf.Team.ID)
	if err != nil {
		log.Printf("%s error getting monitord patient [%v]\n", action, err)
		a.sendModelAsResWithStatus(
			res,
			&status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_MONITORED_PATIENT)},
			http.StatusInternalServerError,
		)
		return
	}
	monitoredEnd := time.Now().Add(90 * 24 * time.Hour)
	if patientMonitored != nil && patientMonitored.TeamID == conf.Team.ID &&
		patientMonitored.Monitoring != nil && patientMonitored.Monitoring.MonitoringEnd != nil {
		monitoredEnd = *patientMonitored.Monitoring.MonitoringEnd
	}
	var patient = store.Patient{
		UserID: conf.UserId,
		TeamID: conf.Team.ID,
		Monitoring: &store.PatientMonitoring{
			MonitoringEnd: &monitoredEnd,
			Status:        "accepted",
		},
	}
	_, err = a.perms.UpdatePatientMonitoringWithContext(req.Context(), a.sl.TokenProvide(), patient)
	if err != nil {
		log.Printf("%s error setting permissions [%v]\n", action, err)
		a.sendModelAsResWithStatus(
			res,
			&status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_DECODING_BODY)},
			http.StatusInternalServerError,
		)
		return
	}

	a.acceptAnyInvite(res, req, conf)
	log.Printf("%s: permissions were set for [%v] after an invite was accepted", action, userid)

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

	// are we updating a team member or a patient
	var err error
	if conf.Role != "patient" {
		member := store.Member{
			UserID:           conf.UserId,
			TeamID:           conf.Team.ID,
			InvitationStatus: "accepted",
			Role:             conf.Role,
		}
		_, err = a.perms.AddTeamMember(a.sl.TokenProvide(), member)
	} else {
		patient := store.Patient{
			UserID:           conf.UserId,
			TeamID:           conf.Team.ID,
			InvitationStatus: "accepted",
		}
		_, err = a.perms.UpdatePatient(getSessionToken(req), patient)
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

	log.Printf("AcceptInvite: permissions were set for [%v -> %v] after an invite was accepted", sanitize(conf.Team.ID), sanitize(conf.UserId))
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

// @Summary Dismiss a team invite
// @Description Invitee or Admin can dismiss a team invite. A patient can dismiss a care team invite.
// @ID hydrophone-api-dismissTeamInvite
// @Accept  json
// @Produce  json
// @Param teamid path string true "Team ID"
// @Param payload body models.Confirmation true "invitation details"
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

	tokenValue := getSessionToken(req)
	// by default you can just act on your records
	dismiss.UserId = userID
	dismiss.Team = &models.Team{ID: teamID}

	if isAdmin, _, err := a.getTeamForUser(nil, tokenValue, teamID, token.UserId, res); isAdmin && err == nil {
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
				patient := store.Patient{
					UserID:           conf.UserId,
					TeamID:           teamID,
					InvitationStatus: "rejected",
				}
				_, err = a.perms.UpdatePatient(tokenValue, patient)
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
				log.Printf("dismiss invite [%s] for [%s]", sanitize(dismiss.Key), sanitize(dismiss.Team.ID))
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

// @Summary Dismiss a monitoring invite
// @Description Patient or Admin can dismiss a monitoring invite.
// @ID hydrophone-api-dismissMonitoringInvite
// @Accept  json
// @Produce  json
// @Param teamid path string true "Team ID"
// @Param userid path string true "User ID"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "teamid or/and userid are missing"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provide sufficient privileges"
// @Failure 404 {object} status.Status "invitation not found"
// @Failure 500 {object} status.Status "Error (internal) while processing the data"
// @Router /dismiss/team/monitoring/{teamid}/{userid} [put]
// @security TidepoolAuth
func (a *Api) DismissMonitoringInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	token := a.token(res, req)
	if token == nil {
		return
	}
	tokenValue := getSessionToken(req)

	teamid := sanitize(vars["teamid"])
	patientid := sanitize(vars["userid"])

	// either the token is the inviteeID or the admin ID
	// let's find out what type of user it is later on
	userid := token.UserId

	// not sure it's necessary
	if teamid == "" || patientid == "" {
		a.sendModelAsResWithStatus(res, &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_INVALID_DATA)}, http.StatusBadRequest)
		return
	}

	dismiss := &models.Confirmation{
		UserId: patientid,
		Team:   &models.Team{ID: teamid},
		Type:   models.TypeMedicalTeamMonitoringInvite,
		Status: models.StatusPending,
	}

	// Needed as the UpdatePatientMonitoringWithContext call will use server token
	//(and because as a patient this route is forbidden)
	if userid != patientid {
		// by default you can just act on your records
		if isAdmin, _, err := a.getTeamForUser(nil, tokenValue, teamid, token.UserId, res); !isAdmin || err != nil {
			// you are not a team admin for the given team
			log.Printf("DismissMonitoring: [%s] not authorized for [%s]", token.UserId, teamid)
			a.sendModelAsResWithStatus(res, &status.StatusError{Status: status.NewStatus(http.StatusUnauthorized, STATUS_UNAUTHORIZED)}, http.StatusUnauthorized)
			return
		}
	}

	if conf, err := a.findExistingConfirmation(req.Context(), dismiss, res); err != nil {
		log.Printf("DismissMonitoring: finding [%s]", err.Error())
		a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
		return
	} else if conf != nil {
		// the pending confirmation exists
		var patient = store.Patient{
			UserID: conf.UserId,
			TeamID: teamid,
			Monitoring: &store.PatientMonitoring{
				MonitoringEnd: nil,
				Status:        "rejected",
			},
		}

		// this one will check permissions and memberships
		_, err = a.perms.UpdatePatientMonitoringWithContext(req.Context(), a.sl.TokenProvide(), patient)
		if err != nil {
			log.Printf("DismissMonitoring error setting permissions [%v]\n", err)
			a.sendModelAsResWithStatus(
				res,
				&status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, err.Error())},
				http.StatusInternalServerError,
			)
			return
		}

		if userid != patientid {
			// canceled by team
			conf.UpdateStatus(models.StatusCanceled)
		} else {
			// declined by patient
			conf.UpdateStatus(models.StatusDeclined)
		}

		if a.addOrUpdateConfirmation(req.Context(), conf, res) {
			log.Printf("User [%s] has dismissed the monitoring in team [%s] for patient [%s]", token.UserId, patient.TeamID, patient.UserID)
			a.logAudit(req, "dismissmonitoring ")
			res.WriteHeader(http.StatusOK)
			return
		}
	}
	statusErr := &status.StatusError{Status: status.NewStatus(http.StatusNotFound, statusInviteNotFoundMessage)}
	log.Printf("DismissMonitoring: [%s]", statusErr.Error())
	a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
}

// @Summary Send invitation to a hcp or patient for joining a medical team
// @Description  create a notification for the invitee and send him an email with the invitation. The patient account has to exist otherwise the invitation is rejected.
// @ID hydrophone-api-SendTeamInvite
// @Accept  json
// @Produce  json
// @Param payload body inviteBody true "invitation details"
// @Success 200 {object} models.Confirmation "invite details"
// @Failure 400 {object} status.Status "userId, teamId and isAdmin were not provided or the payload is missing/malformed"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provide sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 405 {object} status.Status "HCP cannot invite a patient to care team"
// @Failure 409 {object} status.Status "user already has a pending or declined invite OR user is already part of the team"
// @Failure 422 {object} status.Status "Error when sending the email (probably caused by the mailling service"
// @Failure 500 {object} status.Status "Internal error while processing the invite, detailled error returned in the body"
// @Router /send/team/invite [post]
// @security TidepoolAuth
func (a *Api) SendTeamInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	// By default, the invitee language will be "en" for English (as we don't know which language suits him)
	// In case the invitee is a known user, the language will be overriden in a later step
	var inviteeLanguage = GetUserChosenLanguage(req)
	tokenValue := getSessionToken(req)
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

	auth, team, _ := a.getTeamForUser(nil, tokenValue, ib.TeamID, token.UserId, res)

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
		invite.Team = &models.Team{ID: ib.TeamID, Name: team.Name}
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

				emailContent := map[string]string{
					"MedicalteamName":          team.Name,
					"MedicalteamAddress":       formatAddress(team.Address),
					"MedicalteamPhone":         team.Phone,
					"MedicalteamIentification": team.Code,
					"CreatorName":              invite.Creator.Profile.FullName,
					"Email":                    invite.Email,
					"WebPath":                  webPath,
					"Duration":                 invite.GetReadableDuration(),
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

// @Summary Send invitation to a patient for monitoring purpose
// @Description  create a notification for the invitee and send him an email with the invitation to be monitored. The patient account has to exist and the patient has to be a member of the team otherwise the invitation is rejected.
// @ID hydrophone-api-SendMonitoringTeamInvite
// @Accept  json
// @Produce  json
// @Param teamid path string true "Team ID"
// @Param userid path string true "invited user id"
// @Param monitoringInfo body inviteMonitoringBody true "Monitoring info i.e end of monitoring period"
// @Success 200 {object} models.Confirmation "invite details"
// @Failure 400 {object} status.Status "teamId is not found, team is not a monitoring team"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provide sufficient privileges, requesting user is not an admin of the team"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 409 {object} status.Status "user already has a pending or declined invite OR user is already part of the team"
// @Failure 422 {object} status.Status "Error when sending the email (probably caused by the mailling service"
// @Failure 500 {object} status.Status "Internal error while processing the invite, detailled error returned in the body"
// @Router /send/team/monitoring/{teamid}/{userid} [post]
// @security TidepoolAuth
func (a *Api) SendMonitoringTeamInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	tokenValue := getSessionToken(req)
	token := a.token(res, req)
	if token == nil {
		return
	}

	invitorID := token.UserId
	if invitorID == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	var ib = &inviteMonitoringBody{}
	if err := json.NewDecoder(req.Body).Decode(ib); err != nil {
		log.Printf("SendMonitoringTeamInvite: error decoding invite to detail %v\n", err)
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_INVITE)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}
	if ib.MonitoringEnd.IsZero() {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_INVITE)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}

	patientid := sanitize(vars["userid"])
	teamid := sanitize(vars["teamid"])

	// check requesting user is admin of the team
	isTeamAdmin, team, _ := a.getTeamForUser(req.Context(), tokenValue, teamid, token.UserId, res)
	if team.ID == "" {
		return
	}
	if !isTeamAdmin {
		// not an admin of the team
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusUnauthorized, STATUS_NOT_ADMIN)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}
	if team.RemotePatientMonitoring == nil || (team.RemotePatientMonitoring != nil && !*(team.RemotePatientMonitoring).Enabled) {
		// monitoring not enabled for the given team
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_NOT_TEAM_MONITORING)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}

	// check the patient is already a invite and if user is already a patient
	if canBeInvited, invitedUsr := a.checkForMonitoringTeamInviteById(req.Context(), patientid, invitorID, tokenValue, team, res); !canBeInvited {
		log.Printf("SendMonitoringInvite: invited user [%s] cannot be invited", patientid)
		return
	} else if invitedUsr != nil {
		// the user is member of the team and has not yet been invited
		var invite *models.Confirmation
		invite, _ = models.NewConfirmation(
			models.TypeMedicalTeamMonitoringInvite,
			models.TemplateNameMedicalteamMonitoringInvite,
			invitorID)
		invite.Team = &models.Team{ID: teamid, Name: team.Name}
		invite.Status = models.StatusPending
		invite.UserId = invitedUsr.UserID
		invite.Email = invitedUsr.Username
		inviteeLanguage := a.getUserLanguage(invite.UserId, res)

		if a.addOrUpdateConfirmation(req.Context(), invite, res) {
			a.logAudit(req, "monitoring invite created")

			if err := a.addProfile(invite); err != nil {
				log.Println("SendMonitoringInvite: ", err.Error())
			} else {
				emailContent := map[string]string{
					"MedicalteamName":    team.Name,
					"MedicalteamAddress": formatAddress(team.Address),
					"MedicalteamPhone":   team.Phone,
					"CreatorName":        invite.Creator.Profile.FullName,
					"Email":              invite.Email,
					"WebPath":            "notifications",
					"Duration":           invite.GetReadableDuration(),
				}

				if a.createAndSendNotification(req, invite, emailContent, inviteeLanguage) {
					a.logAudit(req, "monitoring invite sent")
				} else {
					a.logAudit(req, "monitoring invite failed to be sent")
					log.Print("Something happened generating a monitoring invite email")
					res.WriteHeader(http.StatusUnprocessableEntity)
					return
				}
			}
			// Updating patient profile with referring doctor
			if ib.ReferringDoctor != nil {
				patientProfile := make(map[string]interface{})
				patientProfile["referringDoctor"] = *ib.ReferringDoctor
				profileUpdate := make(map[string]interface{})
				profileUpdate["patient"] = patientProfile

				err := a.seagull.SetCollection(invitedUsr.UserID, "profile", a.sl.TokenProvide(), profileUpdate)
				if err != nil {
					log.Printf("error updating patient profile [%v]\n", err)
					a.sendModelAsResWithStatus(
						res,
						&status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_MONITORED_PATIENT)},
						http.StatusInternalServerError,
					)
					return
				}

			}
			// Updating crew patient monitoring
			// Default prescription for 90 days
			crewPatient := store.Patient{
				UserID: invitedUsr.UserID,
				TeamID: teamid,
				Monitoring: &store.PatientMonitoring{
					MonitoringEnd: &ib.MonitoringEnd,
					Status:        "pending",
				},
			}
			_, err := a.perms.UpdatePatientMonitoringWithContext(req.Context(), a.sl.TokenProvide(), crewPatient)
			if err != nil {
				log.Printf("error updating crew patient [%v]\n", err)
				a.sendModelAsResWithStatus(
					res,
					&status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_MONITORED_PATIENT)},
					http.StatusInternalServerError,
				)
				return
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
	patient := store.Patient{
		UserID:           invitedUsr.UserID,
		TeamID:           member.TeamID,
		InvitationStatus: member.InvitationStatus,
	}
	if _, err := a.perms.AddPatient(token, patient); err != nil {
		return &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_UPDATING_TEAM)}
	} else {
		log.Printf("Add patient %s in Team %s", invitedUsr.UserID, sanitize(member.TeamID))
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
		log.Printf("Add member %s in Team %s", sanitize(invitedUsr.UserID), sanitize(member.TeamID))
		return nil
	}
}

// @Summary Send notification to an hcp that becomes admin
// @Description  Send an email and a notification to the new admin user. The role change is done but notification is pushed for information (notification status is set to pending). Removing the admin role is managed as an exception. It does not trigger any notification or email.
// @ID hydrophone-api-UpdateTeamRole
// @Accept  json
// @Produce  json
// @Param userid path string true "user id"
// @Param payload body inviteBody true "invitation details"
// @Success 200 {object} models.Confirmation "invite details"
// @Failure 400 {object} status.Status "userId, teamId and isAdmin were not provided or the payload is missing/malformed"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provide sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 409 {object} status.Status "No notification and email sent; User is already an admin"
// @Failure 422 {object} status.Status "Error when sending the email (probably caused by the mailling service"
// @Failure 500 {object} status.Status "Internal error while processing the invite, detailled error returned in the body"
// @Router /send/team/role/{userid} [put]
// @security TidepoolAuth
func (a *Api) UpdateTeamRole(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	// By default, the invitee language will be "en" for Englih (as we don't know which language suits him)
	// In case the invitee is a known user, the language will be overriden in a later step
	var inviteeLanguage = GetUserChosenLanguage(req)
	tokenValue := getSessionToken(req)
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
	var unescapedIb = &inviteBody{}
	if err := json.NewDecoder(req.Body).Decode(unescapedIb); err != nil {
		log.Printf("UpdateInvite: error decoding invite to detail %v\n", err)
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_INVITE)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}
	var ib = checkInviteBody(unescapedIb)

	if ib.TeamID == "" || ib.Email == "" {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_MISSING_DATA_INVITE)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}
	if strings.ToLower(ib.Role) == "admin" {
		ib.Role = "admin"
	} else if strings.ToLower(ib.Role) == "member" {
		ib.Role = "member"
	} else {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_INVALID_DATA)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}

	_, team, err := a.getTeamForUser(nil, tokenValue, ib.TeamID, invitorID, res)
	if err != nil {
		return
	}

	if isMember, _ := a.isTeamMember(inviteeID, team, false); !isMember {
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
		invite.Team.Name = team.Name
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

				emailContent := map[string]string{
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
		}
	}
}

// @Summary Delete hcp from a medical team
// @Description create a notification for the hcp and send him an email to inform him he has been removed from the medical team
// @ID hydrophone-api-DeleteTeamMember
// @Accept  json
// @Produce  json
// @Param userid path string true "user id to remove from team"
// @Param teamid path string true "team id"
// @Param email query string false "email of the user id to remove from team"
// @Success 200 {object} models.Confirmation "delete member"
// @Failure 400 {object} status.Status "userId, teamId and isAdmin were not provided or the payload is missing/malformed"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provide sufficient privileges"
// @Failure 403 {object} status.Status "Authorization token is invalid"
// @Failure 409 {object} status.Status "No notification and email sent; User is not a member"
// @Failure 422 {object} status.Status "Error when sending the email (probably caused by the mailling service"
// @Failure 500 {object} status.Status "Internal error while processing the invite, detailled error returned in the body"
// @Router /send/team/leave/{teamid}/{userid} [delete]
// @security TidepoolAuth
func (a *Api) DeleteTeamMember(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	// By default, the invitee language will be "en" for Englih (as we don't know which language suits him)
	// In case the invitee is a known user, the language will be overriden in a later step
	var inviteeLanguage = GetUserChosenLanguage(req)
	tokenValue := getSessionToken(req)
	token := a.token(res, req)
	if token == nil {
		return
	}

	inviteeID := vars["userid"]
	if inviteeID == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	teamID := vars["teamid"]
	if teamID == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	invitorID := token.UserId
	if invitorID == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	email := req.URL.Query().Get("email")
	if email == "" {
		user, err := a.sl.GetUser(inviteeID, a.sl.TokenProvide())
		if err != nil || user == nil || user.Username == "" {
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_FINDING_USR)}
			a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
			return
		}
		email = user.Username
	}

	_, team, err := a.getTeamForUser(nil, tokenValue, teamID, token.UserId, res)
	if err != nil {
		return
	}

	isMember, teamMember := a.isTeamMember(inviteeID, team, true)
	if !isMember {
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
	invite.Team.ID = teamID
	invite.Team.Name = team.Name
	invite.Email = email
	invite.Role = teamMember.Role
	invite.UserId = inviteeID
	// does the invitee have a preferred language?
	inviteeLanguage = a.getUserLanguage(invite.UserId, res)

	if err := a.perms.RemoveTeamMember(tokenValue, teamID, invite.UserId); err != nil {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_UPDATING_TEAM)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return
	}

	if a.addOrUpdateConfirmation(req.Context(), invite, res) {
		a.logAudit(req, "invite created")

		if err := a.addProfile(invite); err != nil {
			log.Println("SendInvite: ", err.Error())
		} else {

			emailContent := map[string]string{
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
func (a *Api) isTeamMember(userID string, team store.Team, all bool) (bool, *store.Member) {
	for i := 0; i < len(team.Members); i++ {
		if team.Members[i].UserID == userID && (team.Members[i].InvitationStatus == "accepted" || all) {
			return true, &team.Members[i]
		}
	}
	return false, nil
}

//
// return true is the user userID is admin of the Team identified by teamID
// it returns the Team object corresponding to the team
// if any error occurs during the search, it returns an error with the
// related code
func (a *Api) getTeamForUser(ctx context.Context, token, teamID, userID string, res http.ResponseWriter) (bool, store.Team, error) {
	var auth = false
	var team *store.Team
	var err error
	if ctx == nil {
		team, err = a.perms.GetTeam(token, teamID)
	} else {
		team, err = a.perms.GetTeamWithContext(ctx, token, teamID)
	}

	if err != nil {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_FINDING_TEAM)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return auth, store.Team{}, err
	}
	auth = a.isTeamAdmin(userID, *team)
	return auth, *team, nil
}
