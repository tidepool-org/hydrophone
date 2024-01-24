package api

import (
	"context"
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	clinics "github.com/tidepool-org/clinic/client"
	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/hydrophone/models"
)

const CLINIC_ADMIN_ROLE = "CLINIC_ADMIN"

type ClinicInvite struct {
	ShareCode   string                    `json:"shareCode"`
	Permissions commonClients.Permissions `json:"permissions"`
}

// Send a invite to join my team
//
// status: 200 models.Confirmation
// status: 409 statusExistingInviteMessage - user already has a pending or declined invite
// status: 409 statusExistingMemberMessage - user is already part of the team
// status: 409 statusExistingPatientMessage - user is already patient of the clinic
// status: 400
func (a *Api) InviteClinic(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		ctx := req.Context()
		inviterID := vars["userId"]

		if inviterID == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if permissions, err := a.tokenUserHasRequestedPermissions(token, inviterID, commonClients.Permissions{"root": commonClients.Allowed, "custodian": commonClients.Allowed}); err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, err)
			return
		} else if permissions["root"] == nil && permissions["custodian"] == nil {
			a.sendError(ctx, res, http.StatusUnauthorized, STATUS_UNAUTHORIZED)
			return
		}

		defer req.Body.Close()
		var ib = &ClinicInvite{}
		if err := json.NewDecoder(req.Body).Decode(ib); err != nil {
			a.sendError(ctx, res, http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION, err)
			return
		}

		if ib.ShareCode == "" || ib.Permissions == nil {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		shareCode := clinics.ShareCode(ib.ShareCode)
		limit := clinics.Limit(1)
		response, err := a.clinics.ListClinicsWithResponse(ctx, &clinics.ListClinicsParams{
			ShareCode: &shareCode,
			Limit:     &limit,
		})
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CLINIC, err)
			return
		}
		if response.JSON200 == nil || len(*response.JSON200) == 0 {
			a.sendError(ctx, res, http.StatusNotFound, STATUS_ERR_FINDING_CLINIC, err)
			return
		}

		clinic := (*response.JSON200)[0]
		clinicId := *clinic.Id

		patientExists, err := a.checkExistingPatientOfClinic(ctx, clinicId, inviterID)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_USER, err,
				"checking if user is already a patient of clinic")
			return
		}
		if patientExists {
			a.sendError(ctx, res, http.StatusConflict, statusExistingPatientMessage,
				"user is already a patient of clinic")
			return
		}
		existingInvite, err := a.checkForDuplicateClinicInvite(ctx, clinicId, inviterID)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err,
				zap.String("inviterID", inviterID), "clinic already has or had an invite")
			return
		}
		if existingInvite {
			a.sendError(ctx, res, http.StatusConflict, statusExistingInviteMessage,
				zap.String("inviterID", inviterID), err)
			return
		}

		var suppressEmail bool
		if clinic.SuppressedNotifications != nil && clinic.SuppressedNotifications.PatientClinicInvitation != nil {
			suppressEmail = *clinic.SuppressedNotifications.PatientClinicInvitation
		}

		// Get the list of clinicians to send a notification to
		maxClinicians := clinics.Limit(100)
		role := clinics.Role(CLINIC_ADMIN_ROLE)
		params := &clinics.ListCliniciansParams{
			Role:  &role,
			Limit: &maxClinicians,
		}
		listResponse, err := a.clinics.ListCliniciansWithResponse(ctx, clinics.ClinicId(clinicId), params)
		if err != nil || response.StatusCode() != http.StatusOK {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CLINIC, err)
			return
		}
		var recipients []string
		for _, clinician := range *listResponse.JSON200 {
			if clinician.Email != "" {
				recipients = append(recipients, clinician.Email)
			}
		}

		invite, err := models.NewConfirmationWithContext(models.TypeCareteamInvite, models.TemplateNamePatientClinicInvite, inviterID, ib.Permissions)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_CREATING_CONFIRMATION, err)
			return
		}

		invite.ClinicId = clinicId

		// addOrUpdateConfirmation logs and writes a response on errors
		if a.addOrUpdateConfirmation(ctx, invite, res) {
			a.logMetric("invite created", req)

			if err := a.addProfile(invite); err != nil {
				a.logger(ctx).With(zap.Error(err)).Error(STATUS_ERR_ADDING_PROFILE)
				return
			} else if !suppressEmail {
				fullName := invite.Creator.Profile.FullName

				if invite.Creator.Profile.Patient.IsOtherPerson {
					fullName = invite.Creator.Profile.Patient.FullName
				}

				emailContent := map[string]interface{}{
					"CareteamName": fullName,
					"ClinicName":   clinic.Name,
					"WebPath":      "login",
				}

				if a.createAndSendNotification(req, invite, emailContent, recipients...) {
					a.logMetric("invite sent", req)
				}
			}

			a.sendModelAsResWithStatus(ctx, res, invite, http.StatusOK)
			return
		}
	}
}

// Checks do they have an existing invite or are they already a team member
// Or are they an existing user and already in the group?
func (a *Api) checkForDuplicateClinicInvite(ctx context.Context, clinicId, invitorID string) (bool, error) {

	//already has invite from this user?
	invites, err := a.Store.FindConfirmations(
		ctx,
		&models.Confirmation{CreatorId: invitorID, ClinicId: clinicId, Type: models.TypeCareteamInvite},
		models.StatusPending,
	)
	if err != nil {
		return false, err
	}

	if len(invites) > 0 {

		//rule is we cannot send if the invite is not yet expired
		if !invites[0].IsExpired() {
			return true, nil
		}
	}

	return false, nil
}
