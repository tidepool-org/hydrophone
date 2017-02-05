package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"../models"
	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
)

const (
	//Status message we return from the service
	STATUS_EXISTING_INVITE  = "There is already an existing invite"
	STATUS_EXISTING_MEMBER  = "The user is already an existing member"
	STATUS_INVITE_NOT_FOUND = "No matching invite was found"
	STATUS_INVITE_CANCELED  = "Invite has been canceled"
)

type (
	//Invite details for generating a new invite
	inviteBody struct {
		Email       string                    `json:"email"`
		Permissions commonClients.Permissions `json:"permissions"`
	}
)

//Checks do they have an existing invite or are they already a team member
//Or are they an existing user and already in the group?
func (a *Api) checkForDuplicateInvite(inviteeEmail, invitorId, token string, res http.ResponseWriter) (bool, *shoreline.UserData) {

	//already has invite from this user?
	invites, _ := a.Store.FindConfirmations(
		&models.Confirmation{CreatorId: invitorId, Email: inviteeEmail, Type: models.TypeCareteamInvite},
		models.StatusPending,
		models.StatusDeclined,
		models.StatusCompleted,
	)

	if len(invites) > 0 {

		//rule is we cannot send if the invite is before the window opens
		latestInvite := invites[0].Created
		timeWindowOpens := latestInvite.Add(time.Duration(a.Config.InviteTimeoutDays) * 24 * time.Hour)

		if timeWindowOpens.After(time.Now()) {
			log.Println(STATUS_EXISTING_INVITE)
			log.Printf("last invite was [%v] and window opened [%v]", latestInvite, timeWindowOpens)
			statusErr := &status.StatusError{status.NewStatus(http.StatusConflict, STATUS_EXISTING_INVITE)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
			return true, nil
		}
	}

	//already in the group?
	invitedUsr := a.findExistingUser(inviteeEmail, a.sl.TokenProvide())

	if invitedUsr != nil && invitedUsr.UserID != "" {
		if perms, err := a.gatekeeper.UserInGroup(invitedUsr.UserID, invitorId); err != nil {
			log.Printf("error checking if user is in group [%v]", err)
		} else if perms != nil {
			log.Println(STATUS_EXISTING_MEMBER)
			statusErr := &status.StatusError{status.NewStatus(http.StatusConflict, STATUS_EXISTING_MEMBER)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
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
		inviteeId := vars["userid"]

		if inviteeId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}
		// Non-server tokens only legit when for same userid
		if !token.IsServer && inviteeId != token.UserID {
			log.Printf("GetReceivedInvitations %s ", STATUS_UNAUTHORIZED)
			a.sendModelAsResWithStatus(res, status.StatusError{status.NewStatus(http.StatusUnauthorized, STATUS_UNAUTHORIZED)}, http.StatusUnauthorized)
			return
		}

		invitedUsr := a.findExistingUser(inviteeId, req.Header.Get(TP_SESSION_TOKEN))

		//find all oustanding invites were this user is the invite//
		found, err := a.Store.FindConfirmations(&models.Confirmation{Email: invitedUsr.Emails[0], Type: models.TypeCareteamInvite}, models.StatusPending)

		log.Printf("GetReceivedInvitations: found [%d] pending invite(s)", len(found))
		if err != nil {
			log.Printf("GetReceivedInvitations: error [%v] when finding peding invites ", err)
		}

		if invites := a.checkFoundConfirmations(res, found, err); invites != nil {
			a.ensureIdSet(inviteeId, invites)
			log.Printf("GetReceivedInvitations: found and have checked [%d] invites ", len(invites))
			a.logMetric("get received invites", req)
			a.sendModelAsResWithStatus(res, invites, http.StatusOK)
			return
		}
	}
	return
}

//Get the still-pending invitations for a group you own or are an admin of.
//These are the invitations you have sent that have not been accepted.
//There is no way to tell if an invitation has been ignored.
//
// status: 200
// status: 400
func (a *Api) GetSentInvitations(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {

		invitorId := vars["userid"]

		if invitorId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if permissions, err := a.tokenUserHasRequestedPermissions(token, invitorId, commonClients.Permissions{"root": commonClients.Allowed, "custodian": commonClients.Allowed}); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, err)
			return
		} else if permissions["root"] == nil && permissions["custodian"] == nil {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		//find all invites I have sent that are pending or declined
		found, err := a.Store.FindConfirmations(&models.Confirmation{CreatorId: invitorId, Type: models.TypeCareteamInvite}, models.StatusPending, models.StatusDeclined)
		if invitations := a.checkFoundConfirmations(res, found, err); invitations != nil {
			a.logMetric("get sent invites", req)
			a.sendModelAsResWithStatus(res, invitations, http.StatusOK)
			return
		}
	}
	return
}

//Accept the given invite
//
// status: 200 when accepted
// status: 400 when the incoming data is incomplete or incorrect
func (a *Api) AcceptInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	if token := a.token(res, req); token != nil {

		inviteeId := vars["userid"]
		invitorId := vars["invitedby"]

		if inviteeId == "" || invitorId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		// Non-server tokens only legit when for same userid
		if !token.IsServer && inviteeId != token.UserID {
			log.Printf("AcceptInvite %s ", STATUS_UNAUTHORIZED)
			a.sendModelAsResWithStatus(res, status.StatusError{status.NewStatus(http.StatusUnauthorized, STATUS_UNAUTHORIZED)}, http.StatusUnauthorized)
			return
		}

		accept := &models.Confirmation{}
		if err := json.NewDecoder(req.Body).Decode(accept); err != nil {
			log.Printf("AcceptInvite: error decoding invite data: %v\n", err)
			statusErr := &status.StatusError{status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
			return
		}

		if accept.Key == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if conf, err := a.findExistingConfirmation(accept, res); err != nil {
			log.Printf("AcceptInvite: finding [%s]\n", err.Error())
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
			return
		} else if conf != nil {
			//New set the permissions for the invite
			var permissions commonClients.Permissions
			conf.DecodeContext(&permissions)

			if setPerms, err := a.gatekeeper.SetPermissions(inviteeId, invitorId, permissions); err != nil {
				log.Printf("AcceptInvite: permissions  [%v]\n", err)
				statusErr := &status.StatusError{status.NewStatus(http.StatusInternalServerError, STATUS_ERR_DECODING_CONFIRMATION)}
				a.sendModelAsResWithStatus(res, statusErr, http.StatusInternalServerError)
				return
			} else {
				log.Printf("AcceptInvite: permissions were set as [%v] after an invite was accepted", setPerms)
				//we know the user now
				conf.UserId = inviteeId

				conf.UpdateStatus(models.StatusCompleted)
				if a.addOrUpdateConfirmation(conf, res) {
					a.logMetric("acceptinvite", req)
					res.WriteHeader(http.StatusOK)
					res.Write([]byte(STATUS_OK))
					return
				}
			}
		}
		statusErr := &status.StatusError{status.NewStatus(http.StatusNotFound, STATUS_INVITE_NOT_FOUND)}
		log.Printf("AcceptInvite: [%s]", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
		return
	}
	return
}

// Cancel an invite the has been sent to an email address
//
// status: 200 when cancled
// status: 404 STATUS_INVITE_NOT_FOUND
// status: 400 when the incoming data is incomplete or incorrect
func (a *Api) CancelInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {

		invitorId := vars["userid"]
		email := vars["invited_address"]

		if invitorId == "" || email == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if permissions, err := a.tokenUserHasRequestedPermissions(token, invitorId, commonClients.Permissions{"root": commonClients.Allowed, "custodian": commonClients.Allowed}); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, err)
			return
		} else if permissions["root"] == nil && permissions["custodian"] == nil {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		invite := &models.Confirmation{
			Email:     email,
			CreatorId: invitorId,
			Creator:   models.Creator{},
			Type:      models.TypeCareteamInvite,
		}

		if conf, err := a.findExistingConfirmation(invite, res); err != nil {
			log.Printf("CancelInvite: finding [%s]", err.Error())
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
		} else if conf != nil {
			//cancel the invite
			conf.UpdateStatus(models.StatusCanceled)

			if a.addOrUpdateConfirmation(conf, res) {
				a.logMetric("canceled invite", req)
				res.WriteHeader(http.StatusOK)
				return
			}
		}
		statusErr := &status.StatusError{status.NewStatus(http.StatusNotFound, STATUS_INVITE_NOT_FOUND)}
		log.Printf("CancelInvite: [%s]", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
		return
	}
	return
}

// status: 200
// status: 400
func (a *Api) DismissInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {

		inviteeId := vars["userid"]
		invitorId := vars["invitedby"]

		if inviteeId == "" || invitorId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		// Non-server tokens only legit when for same userid
		if !token.IsServer && inviteeId != token.UserID {
			log.Printf("DismissInvite %s ", STATUS_UNAUTHORIZED)
			a.sendModelAsResWithStatus(res, status.StatusError{status.NewStatus(http.StatusUnauthorized, STATUS_UNAUTHORIZED)}, http.StatusUnauthorized)
			return
		}

		dismiss := &models.Confirmation{}
		if err := json.NewDecoder(req.Body).Decode(dismiss); err != nil {
			log.Printf("DismissInvite: error decoding invite to dismiss [%v]", err)
			statusErr := &status.StatusError{status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
			return
		}

		if dismiss.Key == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if conf, err := a.findExistingConfirmation(dismiss, res); err != nil {
			log.Printf("DismissInvite: finding [%s]", err.Error())
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
			return
		} else if conf != nil {

			conf.UpdateStatus(models.StatusDeclined)

			if a.addOrUpdateConfirmation(conf, res) {
				a.logMetric("dismissinvite", req)
				res.WriteHeader(http.StatusOK)
				return
			}
		}
		statusErr := &status.StatusError{status.NewStatus(http.StatusNotFound, STATUS_INVITE_NOT_FOUND)}
		log.Printf("DismissInvite: [%s]", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
		return
	}
	return
}

//Send a invite to join my team
//
// status: 200 models.Confirmation
// status: 409 STATUS_EXISTING_INVITE - user already has a pending or declined invite
// status: 409 STATUS_EXISTING_MEMBER - user is already part of the team
// status: 400
func (a *Api) SendInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {

		invitorId := vars["userid"]

		if invitorId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if permissions, err := a.tokenUserHasRequestedPermissions(token, invitorId, commonClients.Permissions{"root": commonClients.Allowed, "custodian": commonClients.Allowed}); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, err)
			return
		} else if permissions["root"] == nil && permissions["custodian"] == nil {
			a.sendError(res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		defer req.Body.Close()
		var ib = &inviteBody{}
		if err := json.NewDecoder(req.Body).Decode(ib); err != nil {
			log.Printf("SendInvite: error decoding invite to detail %v\n", err)
			statusErr := &status.StatusError{status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
			return
		}

		if ib.Email == "" || ib.Permissions == nil {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if existingInvite, invitedUsr := a.checkForDuplicateInvite(ib.Email, invitorId, req.Header.Get(TP_SESSION_TOKEN), res); existingInvite == true {
			log.Printf("SendInvite: invited [%s] user already has or had an invite", ib.Email)
			return
		} else {
			//None exist so lets create the invite
			invite, _ := models.NewConfirmationWithContext(models.TypeCareteamInvite, models.TemplateNameCareteamInvite, invitorId, ib.Permissions)

			invite.Email = ib.Email
			if invitedUsr != nil {
				invite.UserId = invitedUsr.UserID
			}

			if a.addOrUpdateConfirmation(invite, res) {
				a.logMetric("invite created", req)

				if err := a.addProfile(invite); err != nil {
					log.Println("SendInvite: ", err.Error())
				} else {

					canUpload := ib.Permissions["upload"]

					emailContent := map[string]interface{}{
						"CareteamName":   invite.Creator.Profile.FullName,
						"Key":            invite.Key,
						"Email":          invite.Email,
						"IsExistingUser": invite.UserId != "",
						"ViewOnlyPerms":  canUpload == nil,
					}

					if a.createAndSendNotification(invite, emailContent) {
						a.logMetric("invite sent", req)
					}
				}

				a.sendModelAsResWithStatus(res, invite, http.StatusOK)
				return
			}
		}

	}
	return
}
