package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"./../models"
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
	InviteBody struct {
		Email       string                    `json:"email"`
		Permissions commonClients.Permissions `json:"permissions"`
	}
	//Content used to generate the invite email
	inviteEmailContent struct {
		Key                string
		CareteamName       string
		IsExistingUser     bool
		ViewAndUploadPerms bool
		ViewOnlyPerms      bool
	}
	//Returned invite preview
	InvitePreview struct {
		Key         string          `json:"key"`
		Email       string          `json:"email"`
		Permissions json.RawMessage `json:"permissions"`
		Creator     creator         `json:"creator"`
		Created     time.Time       `json:"created"`
	}
	creator struct {
		Id       string `json:"id"`
		FullName string `json:"fullName"`
	}
)

//Checks do they have an existing invite or are they already a team member
//Or are they an existing user and already in the group?
func (a *Api) checkForDuplicateInvite(inviteeEmail, invitorId, token string, res http.ResponseWriter) (bool, *shoreline.UserData) {

	//already has invite?
	if a.hasExistingConfirmation(inviteeEmail, models.StatusPending, models.StatusDeclined, models.StatusCompleted) {
		log.Println(STATUS_EXISTING_INVITE)
		statusErr := &status.StatusError{status.NewStatus(http.StatusConflict, STATUS_EXISTING_INVITE)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
		return true, nil
	}

	//already in the group?
	invitedUsr := a.findExistingUser(inviteeEmail, token)

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
	if a.checkToken(res, req) {
		inviteeId := vars["userid"]

		if inviteeId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		invitedUsr := a.findExistingUser(inviteeId, req.Header.Get(TP_SESSION_TOKEN))

		//find all oustanding invites were this user is the invite//
		found, err := a.Store.ConfirmationsToUser(inviteeId, invitedUsr.Emails[0], models.StatusPending)
		if invites := a.checkFoundConfirmations(res, found, err); invites != nil {
			a.ensureIdSet(inviteeId, invites)
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
	if a.checkToken(res, req) {

		invitorId := vars["userid"]

		if invitorId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}
		//find all invites I have sent that are pending or declined
		found, err := a.Store.ConfirmationsFromUser(invitorId, models.StatusPending, models.StatusDeclined)
		if invitations := a.checkFoundConfirmations(res, found, err); invitations != nil {
			a.logMetric("get sent invites", req)
			a.sendModelAsResWithStatus(res, invitations, http.StatusOK)
			return
		}
	}
	return
}

// status: 200
// status: 400
func (a *Api) AcceptInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	if a.checkToken(res, req) {

		inviteeId := vars["userid"]
		invitorId := vars["invitedby"]

		if inviteeId == "" || invitorId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		accept := &models.Confirmation{}
		if err := json.NewDecoder(req.Body).Decode(accept); err != nil {
			log.Printf("AcceptInvite Error: %v\n", err)
			statusErr := &status.StatusError{status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
			return
		}

		if accept.Key == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if conf := a.findExistingConfirmation(accept, res); conf != nil {

			//New set the permissions for the invite
			var permissions commonClients.Permissions
			conf.DecodeContext(&permissions)

			if setPerms, err := a.gatekeeper.SetPermissions(inviteeId, invitorId, permissions); err != nil {
				log.Println("Error setting permissions in AcceptInvite ", err)
				statusErr := &status.StatusError{status.NewStatus(http.StatusInternalServerError, STATUS_ERR_DECODING_CONFIRMATION)}
				a.sendModelAsResWithStatus(res, statusErr, http.StatusInternalServerError)
				return
			} else {
				log.Printf("Permissions were set as [%v] after an invite was accepted", setPerms)
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
	}
	return
}

// status: 200
// status: 404
// status: 400
func (a *Api) CancelInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if a.checkToken(res, req) {

		invitorId := vars["userid"]
		email := vars["invited_address"]

		if invitorId == "" || email == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		invite := &models.Confirmation{
			Email:     email,
			CreatorId: invitorId,
			Type:      models.TypeCareteamInvite,
		}

		if conf := a.findExistingConfirmation(invite, res); conf != nil {
			//cancel the invite
			conf.UpdateStatus(models.StatusCanceled)

			if a.addOrUpdateConfirmation(conf, res) {
				a.logMetric("canceled invite", req)
				res.WriteHeader(http.StatusOK)
				return
			}
		}
		statusErr := &status.StatusError{status.NewStatus(http.StatusNotFound, STATUS_INVITE_NOT_FOUND)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
		return
	}
	return
}

// status: 200
// status: 400
func (a *Api) DismissInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if a.checkToken(res, req) {

		inviteeId := vars["userid"]
		invitorId := vars["invitedby"]

		if inviteeId == "" || invitorId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		dismiss := &models.Confirmation{}
		if err := json.NewDecoder(req.Body).Decode(dismiss); err != nil {
			log.Printf("Error decoding invite to dismiss [%v]", err)
			statusErr := &status.StatusError{status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
			return
		}

		if dismiss.Key == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if conf := a.findExistingConfirmation(dismiss, res); conf != nil {

			conf.UpdateStatus(models.StatusDeclined)

			if a.addOrUpdateConfirmation(conf, res) {
				a.logMetric("dismissinvite", req)
				res.WriteHeader(http.StatusOK)
				return
			}
		}
	}
	return
}

//Send a invite to join my team
//Return duplicate if the invited user already has a pending invite
//Return duplicate if the invited user is already part of my team
//
// status: 200 models.Confirmation
// status: 409 STATUS_EXISTING_INVITE
// status: 409 STATUS_EXISTING_MEMBER
// status: 400
func (a *Api) SendInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if a.checkToken(res, req) {

		invitorId := vars["userid"]

		defer req.Body.Close()
		var ib = &InviteBody{}
		if err := json.NewDecoder(req.Body).Decode(ib); err != nil {
			log.Printf("SendInvite error: %v\n", err)
			statusErr := &status.StatusError{status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
			return
		}

		if ib.Email == "" || ib.Permissions == nil {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if existingInvite, invitedUsr := a.checkForDuplicateInvite(ib.Email, invitorId, req.Header.Get(TP_SESSION_TOKEN), res); existingInvite == true {
			log.Println("The invited user already has or had an invite")
			return
		} else {
			//None exist so lets create the invite
			invite, _ := models.NewConfirmationWithContext(models.TypeCareteamInvite, invitorId, ib.Permissions)

			invite.Email = ib.Email
			if invitedUsr != nil {
				invite.UserId = invitedUsr.UserID
			}

			if a.addOrUpdateConfirmation(invite, res) {
				a.logMetric("invite created", req)

				viewOnly := ib.Permissions["upload"] == ""

				up := &profile{}
				if err := a.seagull.GetCollection(invite.CreatorId, "profile", req.Header.Get(TP_SESSION_TOKEN), &up); err != nil {
					log.Printf("Error getting the creators profile [%v] ", err)
				} else {

					emailContent := &inviteEmailContent{
						CareteamName:       up.FullName,
						Key:                invite.Key,
						IsExistingUser:     invite.UserId != "",
						ViewAndUploadPerms: viewOnly == false,
						ViewOnlyPerms:      viewOnly,
					}

					if a.createAndSendNotfication(invite, emailContent, "Invite to join my careteam") {
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

//Get the invite preview you have been sent for this given key
//Note: this is an unsecured call that you require the invote key for
//
// status: 200
// status: 400
func (a *Api) GetInvitePreview(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	inviteKey := vars["key"]

	if inviteKey != "" {
		if invite, err := a.Store.FindConfirmationByKey(inviteKey); invite != nil {
			svrToken := a.sl.TokenProvide()
			up := &profile{}
			if err := a.seagull.GetCollection(invite.CreatorId, "profile", svrToken, &up); err != nil {
				log.Printf("Error getting the creators profile [%v] ", err)
			}

			preview := &InvitePreview{
				Key:         invite.Key,
				Email:       invite.Email,
				Permissions: invite.Context,
				Created:     invite.Created,
			}
			if up.FullName != "" {
				preview.Creator = creator{Id: invite.CreatorId, FullName: up.FullName}
			}
			a.sendModelAsResWithStatus(res, preview, http.StatusOK)
			return

		} else if err != nil {
			log.Printf("GetInvitePreview error finding the invite [%v]", err)
			statusErr := &status.StatusError{status.NewStatus(http.StatusInternalServerError, "Error trying to find invite.")}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusInternalServerError)
			return
		}
		res.WriteHeader(http.StatusNotFound)
		return
	}
	statusErr := &status.StatusError{status.NewStatus(http.StatusBadRequest, "No invite key was given")}
	a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
	return
}
