package api

import (
	"context"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	clinics "github.com/tidepool-org/clinic/client"
	commonClients "github.com/tidepool-org/go-common/clients"

	"github.com/tidepool-org/hydrophone/models"
)

func (a *Api) GetPatientInvites(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		ctx := req.Context()
		clinicId := vars["clinicId"]

		if clinicId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := a.assertClinicMember(ctx, clinicId, token, res); err != nil {
			return
		}

		// find all outstanding invites that are associated to this clinic
		found, err := a.Store.FindConfirmations(ctx, &models.Confirmation{ClinicId: clinicId, Type: models.TypeCareteamInvite}, models.StatusPending)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if len(found) == 0 {
			result := make([]*models.Confirmation, 0)
			a.sendModelAsResWithStatus(res, result, http.StatusOK)
			return
		}
		if invites := a.addProfileInfoToConfirmations(found); invites != nil {
			a.logMetric("get_patient_invites", req)
			a.sendModelAsResWithStatus(res, invites, http.StatusOK)
			a.logger.Debugf("confirmations found and checked: %d", len(invites))
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

		if err := a.assertClinicMember(ctx, clinicId, token, res); err != nil {
			return
		}

		accept := &models.Confirmation{
			ClinicId: clinicId,
			Key:      inviteId,
		}

		conf, err := a.Store.FindConfirmation(req.Context(), accept)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if conf == nil {
			a.sendError(res, http.StatusNotFound, statusInviteNotFoundMessage)
			return
		}

		validationErrors := make([]error, 0)
		conf.ValidateStatus(models.StatusPending, &validationErrors)
		conf.ValidateType(models.TypeCareteamInvite, &validationErrors)
		conf.ValidateClinicID(clinicId, &validationErrors)

		if len(validationErrors) > 0 {
			a.sendError(res, http.StatusForbidden, statusForbiddenMessage,
				zap.Errors("validation-errors", validationErrors))
			return
		}

		patient, err := a.createClinicPatient(ctx, *conf)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_CREATING_PATIENT, err)
			return
		}

		conf.UpdateStatus(models.StatusCompleted)
		if !a.addOrUpdateConfirmation(req.Context(), conf, res) {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION, err)
			return
		}

		a.logMetric("accept_patient_invite", req)
		a.sendModelAsResWithStatus(res, patient, http.StatusOK)
		return
	}
}

// Cancel or dismiss patient invite for a given clinic
func (a *Api) CancelOrDismissPatientInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		ctx := req.Context()
		clinicId := vars["clinicId"]
		inviteId := vars["inviteId"]

		if clinicId == "" || inviteId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		accept := &models.Confirmation{
			ClinicId: clinicId,
			Key:      inviteId,
		}

		conf, err := a.Store.FindConfirmation(req.Context(), accept)
		if err != nil {
			a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if conf == nil {
			a.sendError(res, http.StatusForbidden, statusInviteNotFoundMessage)
			return
		}

		updatedStatus := models.StatusCanceled
		if token.UserID != conf.CreatorId {
			updatedStatus = models.StatusDeclined
			if err := a.assertClinicMember(ctx, clinicId, token, res); err != nil {
				return
			}
		}

		validationErrors := make([]error, 0)
		conf.ValidateStatusIn([]models.Status{models.StatusPending, models.StatusDeclined}, &validationErrors)
		conf.ValidateType(models.TypeCareteamInvite, &validationErrors)
		conf.ValidateClinicID(clinicId, &validationErrors)

		if len(validationErrors) > 0 {
			a.sendError(res, http.StatusForbidden, statusForbiddenMessage,
				zap.Errors("validation-errors", validationErrors))
			return
		}

		conf.UpdateStatus(updatedStatus)
		if !a.addOrUpdateConfirmation(req.Context(), conf, res) {
			return
		}

		if conf.Status == models.StatusDeclined {
			a.logMetric("decline_patient_invite", req)
		} else if conf.Status == models.StatusCanceled {
			a.logMetric("cancel_clinic_invite", req)
		}

		a.sendModelAsResWithStatus(res, conf, http.StatusOK)
		return
	}
}

func (a *Api) createClinicPatient(ctx context.Context, confirmation models.Confirmation) (*clinics.Patient, error) {
	var permissions commonClients.Permissions
	if err := confirmation.DecodeContext(&permissions); err != nil {
		return nil, err
	}

	body := clinics.CreatePatientFromUserJSONRequestBody{
		Permissions: &clinics.PatientPermissions{
			View:   getPermission(permissions, "view"),
			Upload: getPermission(permissions, "upload"),
			Note:   getPermission(permissions, "note"),
		},
	}

	var patient *clinics.Patient
	clinicId := clinics.ClinicId(confirmation.ClinicId)
	patientId := clinics.PatientId(confirmation.CreatorId)
	response, err := a.clinics.CreatePatientFromUserWithResponse(ctx, clinicId, patientId, body)
	if err != nil {
		return nil, err
	} else if response.StatusCode() == http.StatusConflict {
		patientResponse, err := a.clinics.GetPatientWithResponse(ctx, clinicId, patientId)
		if err != nil {
			return nil, err
		}
		if patientResponse.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("unexpected status code %v when fetching patient by id", patientResponse.StatusCode())
		}
		patient = patientResponse.JSON200
	} else if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %v when creating patient from existing user", response.StatusCode())
	} else {
		patient = response.JSON200
	}

	a.logger.With(zap.Any("perms", patient.Permissions)).Info("permissions set")
	return patient, nil
}

func getPermission(permissions commonClients.Permissions, permission string) *map[string]interface{} {
	if _, ok := permissions[permission]; ok {
		perm := make(map[string]interface{}, 0)
		return &perm
	}
	return nil
}
