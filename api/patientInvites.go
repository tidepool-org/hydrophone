package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"

	clinics "github.com/tidepool-org/clinic/client"
	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/status"
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

		// assertClinicMember logs and writes a response on errors
		if err := a.assertClinicMember(ctx, clinicId, token, res); err != nil {
			return
		}

		// find all outstanding invites that are associated to this clinic
		found, err := a.Store.FindConfirmations(ctx, &models.Confirmation{ClinicId: clinicId, Type: models.TypeCareteamInvite}, models.StatusPending)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if len(found) == 0 {
			result := make([]*models.Confirmation, 0)
			a.sendModelAsResWithStatus(ctx, res, result, http.StatusOK)
			return
		}
		if invites := a.addProfileInfoToConfirmations(ctx, found); invites != nil {
			a.logMetric("get_patient_invites", req)
			a.sendModelAsResWithStatus(ctx, res, invites, http.StatusOK)
			a.logger(ctx).Debugf("confirmations found and checked: %d", len(invites))
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

		// addOrUpdateConfirmation logs and writes a response on errors
		if err := a.assertClinicMember(ctx, clinicId, token, res); err != nil {
			return
		}

		c := &models.Confirmation{
			ClinicId: clinicId,
			Key:      inviteId,
		}
		conf, err := a.Store.FindConfirmation(ctx, c)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if conf == nil {
			a.sendError(ctx, res, http.StatusNotFound, statusInviteNotFoundMessage)
			return
		}

		validationErrors := make([]error, 0)
		conf.ValidateStatus(models.StatusPending, &validationErrors)
		conf.ValidateType(models.TypeCareteamInvite, &validationErrors)
		conf.ValidateClinicID(clinicId, &validationErrors)

		if len(validationErrors) > 0 {
			a.sendError(ctx, res, http.StatusForbidden, statusForbiddenMessage,
				zap.Errors("validation-errors", validationErrors))
			return
		}

		accept := models.AcceptPatientInvite{}
		if req.ContentLength > 0 {
			if err := json.NewDecoder(req.Body).Decode(&accept); err != nil {
				a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_CREATING_PATIENT,
					fmt.Errorf("error decoding accept patient invite body: %w", err),
				)
				return
			}
		}

		mrnRequired, err := a.isMRNRequired(ctx, conf.ClinicId)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_CREATING_PATIENT,
				fmt.Errorf("error fetching mrn requirement settings: %w", err),
			)
			return
		}
		if mrnRequired && strings.TrimSpace(accept.MRN) == "" {
			a.sendModelAsResWithStatus(
				ctx,
				res,
				&status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_MRN_REQUIRED)},
				http.StatusBadRequest,
			)
			return
		}

		patient, err := a.createClinicPatient(ctx, *conf, accept)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_CREATING_PATIENT, err)
			return
		}

		conf.UpdateStatus(models.StatusCompleted)
		// addOrUpdateConfirmation logs and writes a response on errors
		if !a.addOrUpdateConfirmation(ctx, conf, res) {
			return
		}

		a.logMetric("accept_patient_invite", req)
		a.sendModelAsResWithStatus(ctx, res, patient, http.StatusOK)
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

		conf, err := a.Store.FindConfirmation(ctx, accept)
		if err != nil {
			a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION, err)
			return
		}
		if conf == nil {
			a.sendError(ctx, res, http.StatusForbidden, statusInviteNotFoundMessage)
			return
		}

		updatedStatus := models.StatusCanceled
		if token.UserID != conf.CreatorId {
			updatedStatus = models.StatusDeclined
			// assertClinicMember logs and writes a response on errors
			if err := a.assertClinicMember(ctx, clinicId, token, res); err != nil {
				return
			}
		}

		validationErrors := make([]error, 0)
		conf.ValidateStatusIn([]models.Status{models.StatusPending, models.StatusDeclined}, &validationErrors)
		conf.ValidateType(models.TypeCareteamInvite, &validationErrors)
		conf.ValidateClinicID(clinicId, &validationErrors)

		if len(validationErrors) > 0 {
			a.sendError(ctx, res, http.StatusForbidden, statusForbiddenMessage,
				zap.Errors("validation-errors", validationErrors))
			return
		}

		conf.UpdateStatus(updatedStatus)
		// addOrUpdateConfirmation logs and writes a response on errors
		if !a.addOrUpdateConfirmation(ctx, conf, res) {
			return
		}

		if conf.Status == models.StatusDeclined {
			a.logMetric("decline_patient_invite", req)
		} else if conf.Status == models.StatusCanceled {
			a.logMetric("cancel_clinic_invite", req)
		}

		a.sendModelAsResWithStatus(ctx, res, conf, http.StatusOK)
		return
	}
}

func (a *Api) createClinicPatient(ctx context.Context, confirmation models.Confirmation, accept models.AcceptPatientInvite) (*clinics.PatientV1, error) {
	var permissions commonClients.Permissions
	if err := confirmation.DecodeContext(&permissions); err != nil {
		return nil, err
	}

	body := clinics.CreatePatientFromUserJSONRequestBody{
		Permissions: &clinics.PatientPermissionsV1{
			View:   getPermission(permissions, "view"),
			Upload: getPermission(permissions, "upload"),
			Note:   getPermission(permissions, "note"),
		},
	}
	if accept.BirthDate != "" {
		body.BirthDate = &types.Date{}
		if err := body.BirthDate.UnmarshalText([]byte(accept.BirthDate)); err != nil {
			return nil, err
		}
	}
	if accept.FullName != "" {
		body.FullName = &accept.FullName
	}
	if accept.MRN != "" {
		body.Mrn = &accept.MRN
	}
	if count := len(accept.Tags); count > 0 {
		tagIds := make(clinics.PatientTagIdsV1, 0, count)
		for _, tag := range accept.Tags {
			tagIds = append(tagIds, tag)
		}
		body.Tags = &tagIds
	}
	if count := len(accept.Sites); count > 0 {
		sites := make([]clinics.SiteV1, 0, count)
		for _, site := range accept.Sites {
			sites = append(sites, clinics.SiteV1{
				Id:   site.Id,
				Name: site.Name,
			})
		}
		body.Sites = &sites
	}

	var patient *clinics.PatientV1
	clinicId := confirmation.ClinicId
	patientId := confirmation.CreatorId
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

	a.logger(ctx).With(zap.Any("perms", patient.Permissions)).Info("permissions set")
	return patient, nil
}

func (a *Api) isMRNRequired(ctx context.Context, clinicId string) (bool, error) {
	response, err := a.clinics.GetMRNSettingsWithResponse(ctx, clinicId)
	if err != nil {
		return false, err
	}

	// The clinic doesn't have custom MRN settings
	if response.StatusCode() == http.StatusNotFound {
		return false, nil
	} else if response.StatusCode() != http.StatusOK {
		return false, fmt.Errorf("unexpected response code when fetching clinic %s settings %v", clinicId, response.StatusCode())
	}

	return response.JSON200.Required, nil
}

func getPermission(permissions commonClients.Permissions, permission string) *map[string]interface{} {
	if _, ok := permissions[permission]; ok {
		perm := make(map[string]interface{}, 0)
		return &perm
	}
	return nil
}
