package api

import (
	"context"
	"fmt"
	clinics "github.com/tidepool-org/clinic/client"
	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/hydrophone/models"
	"go.uber.org/zap"
	"net/http"
)

func (a *Api) GetPatientInvites(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		if !a.Config.ClinicServiceEnabled {
			a.sendError(res, http.StatusNotFound, statusInviteNotFoundMessage)
			return
		}

		ctx := req.Context()
		clinicId := vars["clinicId"]

		if clinicId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := a.assertClinicMember(ctx, clinicId, token, res); err != nil {
			a.logger.Errorw("token owner is not a clinic member", zap.Error(err))
			return
		}

		// find all outstanding invites that are associated to this clinic
		found, err := a.Store.FindConfirmations(ctx, &models.Confirmation{ClinicId: clinicId, Type: models.TypeCareteamInvite}, models.StatusPending)
		if invites := a.checkFoundConfirmations(res, found, err); invites != nil {
			a.logger.Infof("found and checked %d confirmations", len(invites))
			a.logMetric("get_patient_invites", req)
			a.sendModelAsResWithStatus(res, invites, http.StatusOK)
			return
		}
	}
}

// Accept patient invite for a given clinic
func (a *Api) AcceptPatientInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if token := a.token(res, req); token != nil {
		if !a.Config.ClinicServiceEnabled {
			a.sendError(res, http.StatusNotFound, statusInviteNotFoundMessage)
			return
		}

		ctx := req.Context()
		clinicId := vars["clinicId"]
		inviteId := vars["inviteId"]

		if clinicId == "" || inviteId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := a.assertClinicMember(ctx, clinicId, token, res); err != nil {
			a.logger.Errorw("token owner is not a clinic member", zap.Error(err))
			return
		}

		accept := &models.Confirmation{
			ClinicId: clinicId,
			Key:      inviteId,
		}

		conf, err := a.findExistingConfirmation(req.Context(), accept, res)
		if err != nil {
			a.logger.Errorw("error while finding confirmation", zap.Error(err))
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
			return
		}
		if conf == nil {
			a.logger.Warn("confirmation not found")
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusNotFound, statusInviteNotFoundMessage)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
			return
		}

		validationErrors := make([]error, 0)
		conf.ValidateStatus(models.StatusPending, &validationErrors)
		conf.ValidateType(models.TypeCareteamInvite, &validationErrors)
		conf.ValidateClinicID(clinicId, &validationErrors)

		if len(validationErrors) > 0 {
			for _, validationError := range validationErrors {
				a.logger.Warnw("forbidden as there was a expectation mismatch", zap.Error(validationError))
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
			a.logger.Errorw("error creating patient", zap.Error(err))
			a.sendModelAsResWithStatus(
				res,
				&status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_CREATING_PATIENT)},
				http.StatusInternalServerError,
			)
			return
		}

		conf.UpdateStatus(models.StatusCompleted)
		if !a.addOrUpdateConfirmation(req.Context(), conf, res) {
			a.logger.Warn("error adding or updating confirmation")
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusInternalServerError)
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
		if !a.Config.ClinicServiceEnabled {
			a.sendError(res, http.StatusNotFound, statusInviteNotFoundMessage)
			return
		}

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
			Status:   models.StatusPending,
		}

		conf, err := a.findExistingConfirmation(req.Context(), accept, res)
		if err != nil {
			a.logger.Errorw("error while finding confirmation", zap.Error(err))
			a.sendModelAsResWithStatus(res, err, http.StatusInternalServerError)
			return
		}
		if conf == nil {
			a.logger.Warn("confirmation not found")
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusForbidden, statusForbiddenMessage)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusForbidden)
			return
		}

		updatedStatus := models.StatusCanceled
		if token.UserID != conf.CreatorId {
			updatedStatus = models.StatusDeclined
			if err := a.assertClinicMember(ctx, clinicId, token, res); err != nil {
				a.logger.Errorw("token owner is not a clinic member", zap.Error(err))
				return
			}
		}

		validationErrors := make([]error, 0)
		conf.ValidateStatus(models.StatusPending, &validationErrors)
		conf.ValidateType(models.TypeCareteamInvite, &validationErrors)
		conf.ValidateClinicID(clinicId, &validationErrors)

		if len(validationErrors) > 0 {
			for _, validationError := range validationErrors {
				a.logger.Warnw("forbidden as there was a expectation mismatch", zap.Error(validationError))
			}
			a.sendModelAsResWithStatus(
				res,
				&status.StatusError{Status: status.NewStatus(http.StatusForbidden, statusForbiddenMessage)},
				http.StatusForbidden,
			)
			return
		}

		conf.UpdateStatus(updatedStatus)
		if !a.addOrUpdateConfirmation(req.Context(), conf, res) {
			a.logger.Warn("error adding or updating confirmation")
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusInternalServerError)
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

	a.logger.Infof("permissions were set as [%v] after an invite was accepted", patient.Permissions)
	return patient, nil
}

func getPermission(permissions commonClients.Permissions, permission string) *map[string]interface{} {
	if _, ok := permissions[permission]; ok {
		perm := make(map[string]interface{}, 0)
		return &perm
	}
	return nil
}
