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
	EXISTING_INVITE = "There is already an existing invite"
	EXISTING_MEMBER = "The user is already an existing member"
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
		log.Println(EXISTING_INVITE)
		statusErr := &status.StatusError{status.NewStatus(http.StatusConflict, EXISTING_INVITE)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
		return true, nil
	}

	//already in the group?
	invitedUsr := a.findExistingUser(inviteeEmail, token)

	if invitedUsr != nil && invitedUsr.UserID != "" {
		if perms, err := a.gatekeeper.UserInGroup(invitedUsr.UserID, invitorId); err != nil {
			log.Printf("error checking if user is in group [%v]", err)
		} else if perms != nil {
			log.Println(EXISTING_MEMBER)
			statusErr := &status.StatusError{status.NewStatus(http.StatusConflict, EXISTING_MEMBER)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
			return true, invitedUsr
		}
		return false, invitedUsr
	}
	return false, nil
}

//Get the invite preview you have been sent for this given key
//Note: this is an unsecured call that you require the invote key for
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

	res.WriteHeader(http.StatusBadRequest)
	return
}
