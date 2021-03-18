package api

import (
	"context"
	"encoding/json"
	"fmt"
	api "github.com/tidepool-org/clinic/client"
	"log"
	"net/http"

	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/hydrophone/models"
)

const (
	//Status message we return from the service
	statusExistingInviteMessage  = "There is already an existing invite"
	statusExistingMemberMessage  = "The user is already an existing member"
	statusExistingPatientMessage = "The user is already a patient of the clinic"
	statusInviteNotFoundMessage  = "No matching invite was found"
	statusInviteCanceledMessage  = "Invite has been canceled"
	statusForbiddenMessage       = "Forbidden to perform requested operation"
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
func (a *Api) checkForDuplicateInvite(ctx context.Context, inviteeEmail, invitorID string, res http.ResponseWriter) bool {

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
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusConflict, statusExistingInviteMessage)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
			return true
		}
	}

	return false
}

func (a *Api) checkExistingPatientOfClinic(ctx context.Context, clinicId, patientId string) (bool, error) {
	return false, nil
}

func (a *Api) 	checkAccountAlreadySharedWithUser(invitorID, inviteeEmail string, res http.ResponseWriter) (bool, *shoreline.UserData) {
	//already in the group?
	invitedUsr := a.findExistingUser(inviteeEmail, a.sl.TokenProvide())

	if invitedUsr != nil && invitedUsr.UserID != "" {
		if perms, err := a.gatekeeper.UserInGroup(invitedUsr.UserID, invitorID); err != nil {
			log.Printf("error checking if user is in group [%v]", err)
		} else if perms != nil {
			log.Println(statusExistingMemberMessage)
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusConflict, statusExistingMemberMessage)}
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
		ctx := req.Context()
		inviteeID := vars["userid"]

		if inviteeID == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}
		// Non-server tokens only legit when for same userid
		if !token.IsServer && inviteeID != token.UserID {
			log.Printf("GetReceivedInvitations %s ", STATUS_UNAUTHORIZED)
			a.sendModelAsResWithStatus(res, status.StatusError{Status: status.NewStatus(http.StatusUnauthorized, STATUS_UNAUTHORIZED)}, http.StatusUnauthorized)
			return
		}

		// Return an empty list if there is a clinic with the same email address
		invitedUsr := a.findExistingUser(inviteeID, req.Header.Get(TP_SESSION_TOKEN))
		if clinic, err := a.findExistingClinic(ctx, invitedUsr.Emails[0]); err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CLINIC, err)
			return
		} else if clinic != nil {
			a.sendModelAsResWithStatus(res, []*models.Confirmation{}, http.StatusOK)
			return
		}

		//find all oustanding invites were this user is the invite//
		found, err := a.Store.FindConfirmations(req.Context(), &models.Confirmation{Email: invitedUsr.Emails[0], Type: models.TypeCareteamInvite}, models.StatusPending)

		//log.Printf("GetReceivedInvitations: found [%d] pending invite(s)", len(found))
		if err != nil {
			log.Printf("GetReceivedInvitations: error [%v] when finding peding invites ", err)
		}

		if invites := a.checkFoundConfirmations(res, found, err); invites != nil {
			a.ensureIdSet(req.Context(), inviteeID, invites)
			log.Printf("GetReceivedInvitations: found and have checked [%d] invites ", len(invites))
			a.logMetric("get received invites", req)
			a.sendModelAsResWithStatus(res, invites, http.StatusOK)
			return
		}
	}
}

func (a *Api) GetPatientInvites(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		ctx := req.Context()
		clinicId := vars["clinicId"]

		if clinicId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := a.assertClinicAdmin(ctx, clinicId, token, res); err != nil {
			log.Println(err.Error())
			return
		}

		//find all oustanding invites that are associated to this clinic
		found, err := a.Store.FindConfirmations(ctx, &models.Confirmation{ClinicId: clinicId, Type: models.TypeCareteamInvite}, models.StatusPending)
		if invites := a.checkFoundConfirmations(res, found, err); invites != nil {
			log.Printf("GetPatientInvitations: found and have checked [%d] invites ", len(invites))
			a.logMetric("get_patient_invites", req)
			a.sendModelAsResWithStatus(res, invites, http.StatusOK)
			return
		}
	}
}

// Accept patient invite for a given clinic
func (a *Api) AcceptPatientInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		ctx := req.Context()
		clinicId := vars["clinicId"]
		inviteId := vars["inviteId"]

		if clinicId == "" || inviteId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := a.assertClinicAdmin(ctx, clinicId, token, res); err != nil {
			log.Println(err.Error())
			return
		}

		accept := &models.Confirmation{
			ClinicId: clinicId,
			Key:      inviteId,
		}

		conf, err := a.findExistingConfirmation(req.Context(), accept, res)
		if err != nil {
			log.Printf("AcceptPatientInvite error while finding confirmation [%s]\n", err.Error())
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
			return
		}
		if conf == nil {
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusNotFound, statusInviteNotFoundMessage)}
			log.Println("AcceptPatientInvite ", statusErr.Error())
			a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
			return
		}

		validationErrors := make([]error, 0)
		conf.ValidateStatus(models.StatusPending, &validationErrors).
			ValidateType(models.TypeCareteamInvite, &validationErrors).
			ValidateClinicID(clinicId, &validationErrors)

		if len(validationErrors) > 0 {
			for _, validationError := range validationErrors {
				log.Println("AcceptPatientInvite forbidden as there was a expectation mismatch", validationError)
			}
			a.sendModelAsResWithStatus(
				res,
				&status.StatusError{Status: status.NewStatus(http.StatusForbidden, statusForbiddenMessage)},
				http.StatusForbidden,
			)
			return
		}

		patient, err := a.createClinicPatient(ctx, *conf)
		if err != nil {
			log.Printf("AcceptPatientInvite error creating patient [%v]\n", err)
			a.sendModelAsResWithStatus(
				res,
				&status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_CREATING_PATIENT)},
				http.StatusInternalServerError,
			)
			return
		}

		log.Printf("AcceptPatientInvite: permissions were set as [%v] after an invite was accepted", patient.Permissions)
		conf.UpdateStatus(models.StatusCompleted)
		if !a.addOrUpdateConfirmation(req.Context(), conf, res) {
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION)}
			log.Println("AcceptInvite ", statusErr.Error())
			a.sendModelAsResWithStatus(res, statusErr, http.StatusInternalServerError)
			return
		}

		a.logMetric("accept_patient_invite", req)
		a.sendModelAsResWithStatus(res, patient, http.StatusOK)
		return
	}
}

func (a *Api) createClinicPatient(ctx context.Context, confirmation models.Confirmation) (*api.Patient, error) {
	var permissions commonClients.Permissions
	if err := confirmation.DecodeContext(&permissions); err != nil {
		return nil, err
	}

	body := api.CreatePatientFromUserJSONRequestBody{
		Permissions: &api.PatientPermissions{
			View: getPermission(permissions, "view"),
			Upload: getPermission(permissions, "upload"),
			Note: getPermission(permissions, "note"),
		},
	}

	response, err := a.clinics.CreatePatientFromUserWithResponse(ctx, confirmation.ClinicId, confirmation.CreatorId, body)
	if err != nil {
		return nil, err
	} else if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %v when creating patient from existing user", response.StatusCode())
	}

	return response.JSON200, nil
}

func getPermission(permissions commonClients.Permissions, permission string) *map[string]interface{} {
	if _, ok := permissions[permission]; ok {
		perm := make(map[string]interface{}, 0)
		return &perm
	}
	return nil
}

func (a *Api) assertClinicAdmin(ctx context.Context, clinicId string, token *shoreline.TokenData, res http.ResponseWriter) error {
	// Non-server tokens only legit when for same userid
	if !token.IsServer {
		if result, err := a.clinics.GetClinicianWithResponse(ctx, clinicId, token.UserID); err != nil || result.StatusCode() == http.StatusInternalServerError {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, err)
			return err
		} else if result.StatusCode() != http.StatusOK {
			a.sendModelAsResWithStatus(res, status.StatusError{Status: status.NewStatus(http.StatusUnauthorized, STATUS_UNAUTHORIZED)}, http.StatusUnauthorized)
			return fmt.Errorf("unexpected status code %v when fetching clinician %v from clinic %v", result.StatusCode(), clinicId, token.UserID)
		} else {
			clinician := result.JSON200
			for _, role := range clinician.Roles {
				if role == "CLINIC_ADMIN" {
					return nil
				}
			}
			a.sendModelAsResWithStatus(res, status.StatusError{Status: status.NewStatus(http.StatusUnauthorized, STATUS_UNAUTHORIZED)}, http.StatusUnauthorized)
			return fmt.Errorf("the clinician doesn't have the required permissions %v", clinician.Roles)
		}
	}
	return nil
}

//Get the still-pending invitations for a group you own or are an admin of.
//These are the invitations you have sent that have not been accepted.
//There is no way to tell if an invitation has been ignored.
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

//Accept the given invite
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

		var permissions commonClients.Permissions
		conf.DecodeContext(&permissions)
		setPerms, err := a.gatekeeper.SetPermissions(inviteeID, invitorID, permissions)
		if err != nil {
			log.Printf("AcceptInvite error setting permissions [%v]\n", err)
			a.sendModelAsResWithStatus(
				res,
				&status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_DECODING_CONFIRMATION)},
				http.StatusInternalServerError,
			)
			return
		}
		log.Printf("AcceptInvite: permissions were set as [%v] after an invite was accepted", setPerms)
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

		if conf, err := a.findExistingConfirmation(req.Context(), invite, res); err != nil {
			log.Printf("CancelInvite: finding [%s]", err.Error())
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
		} else if conf != nil {
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

		if conf, err := a.findExistingConfirmation(req.Context(), dismiss, res); err != nil {
			log.Printf("DismissInvite: finding [%s]", err.Error())
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
			return
		} else if conf != nil {

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

//Send a invite to join my team
//
// status: 200 models.Confirmation
// status: 409 statusExistingInviteMessage - user already has a pending or declined invite
// status: 409 statusExistingMemberMessage - user is already part of the team
// status: 409 statusExistingPatientMessage - user is already patient of the clinic
// status: 400
func (a *Api) SendInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		ctx := req.Context()
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

		defer req.Body.Close()
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

		if existingInvite := a.checkForDuplicateInvite(req.Context(), ib.Email, invitorID, res); existingInvite {
			log.Printf("SendInvite: invited [%s] user already has or had an invite", ib.Email)
			return
		}

		invite, _ := models.NewConfirmationWithContext(models.TypeCareteamInvite, models.TemplateNameCareteamInvite, invitorID, ib.Permissions)
		invite.Email = ib.Email

		clinic, err := a.findExistingClinic(ctx, ib.Email)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, err)
			return
		}

		if clinic != nil {
			invite.ClinicId = string(clinic.Id)
			patientExists, err := a.checkExistingPatientOfClinic(ctx, invitorID, invite.ClinicId)
			if err != nil {
				a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, err)
				return
			}
			if patientExists {
				log.Println(statusExistingMemberMessage)
				statusErr := &status.StatusError{Status: status.NewStatus(http.StatusConflict, statusExistingPatientMessage)}
				a.sendModelAsResWithStatus(res, statusErr, http.StatusConflict)
				return
			}
		} else if alreadyMember, invitedUsr := a.checkAccountAlreadySharedWithUser(invitorID, ib.Email, res); alreadyMember {
			log.Printf("SendInvite: invited [%s] user is already a member of the care team", ib.Email)
			return
		} else {
			if invitedUsr != nil {
				invite.UserId = invitedUsr.UserID
			}
		}

		if a.addOrUpdateConfirmation(req.Context(), invite, res) {
			a.logMetric("invite created", req)

			if err := a.addProfile(invite); err != nil {
				log.Println("SendInvite: ", err.Error())
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
					a.logMetric("invite sent", req)
				}
			}

			a.sendModelAsResWithStatus(res, invite, http.StatusOK)
			return
		}
	}
}
