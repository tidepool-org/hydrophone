package api

import (
	"encoding/json"
	"net/http"

	"github.com/mdblp/go-common/clients/status"
	"github.com/mdblp/hydrophone/models"
	log "github.com/sirupsen/logrus"
)

type PrescriptionBody struct {
	Id            string `json:"id"`
	Code          string `json:"code"`
	PatientEmail  string `json:"patientEmail"`
	PatientId     string `json:"patientId"`
	PrescriptorId string `json:"prescriptorId"`
	Product       string `json:"product"`
}

var (
	STATUS_WRONG_NOTIFICATION_TOPIC = "wrong notification topic"
	STATUS_WRONG_APP_PRESCRIPTION   = "missing information in prescription body"
)

// @Summary Send a notification by email
// @Description Create a generic notification, send an email using the template matching the topic provided in the notification url
// @Description this route is only accessible for token servers
// @ID hydrophone-api-SendNotification
// @Accept  json
// @Produce  json
// @Param topic path string true "topic label"
// @Success 200 {array} models.Confirmation
// @Failure 400 {object} status.Status "usereid was not provided"
// @Failure 401 {object} status.Status "Authorization token is missing or does not provided sufficient privileges"
// @Failure 403 {object} status.Status "Not authorized to perform this action, probably you are not a server"
// @Failure 500 {object} status.Status "Internal error"
// @Router /notifications/{topic} [get]
// @security TidepoolAuth
func (a *Api) CreateNotification(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	// Only servers can send notifs
	token := a.token(res, req)
	if token == nil {
		return
	}
	if !token.IsServer {
		a.sendModelAsResWithStatus(
			res,
			&status.StatusError{Status: status.NewStatus(http.StatusForbidden, STATUS_UNAUTHORIZED)},
			http.StatusForbidden,
		)
		return
	}
	// Get topic
	topic := vars["topic"]
	var notif *models.Confirmation
	var emailContent map[string]string

	switch topic {
	case "submit_app_prescription":
		{
			notif, emailContent = a.createAppPrescription(res, req)
			if notif == nil {
				return
			}
		}
	default:
		{
			a.sendModelAsResWithStatus(
				res,
				&status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_WRONG_NOTIFICATION_TOPIC)},
				http.StatusBadRequest,
			)
			return
		}
	}
	a.processNotification(res, req, emailContent, notif)
}

//Prepare email content and confirm object for topic "create prescription"
func (a *Api) createAppPrescription(res http.ResponseWriter, req *http.Request) (*models.Confirmation, map[string]string) {
	presc := &PrescriptionBody{}
	if err := json.NewDecoder(req.Body).Decode(presc); err != nil {
		log.Printf("CreateAppPrescription: error decoding presc to create [%v]", err)
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_NOTIFICATION)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
		return nil, nil
	}

	if presc.PatientEmail == "" || presc.Id == "" || presc.PrescriptorId == "" {
		a.sendModelAsResWithStatus(
			res,
			&status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_WRONG_APP_PRESCRIPTION)},
			http.StatusBadRequest,
		)
		return nil, nil
	}
	notif, _ := models.NewConfirmationWithContext(models.TypeNotification, models.TemplateNameAppPrescription, presc.PrescriptorId, presc)

	// if the invitee is already a Tidepool user, we can use his preferences
	notif.Email = presc.PatientEmail

	emailContent := map[string]string{
		"Product": presc.Product,
		// Prescription code may not be required
		"PrescriptionCode": presc.Code,
		"WebPath":          "prescriptions",
	}
	return notif, emailContent
}

// Send a notification email based on the given email content and confirmation model
func (a *Api) processNotification(res http.ResponseWriter, req *http.Request, content map[string]string, invite *models.Confirmation) {
	var inviteeLanguage = "en"
	creatorMetaData, err := a.seagull.GetCollections(req.Context(), invite.CreatorId, []string{"preferences", "profile"}, a.sl.TokenProvide())
	if err != nil {
		a.sendError(res, http.StatusInternalServerError, STATUS_ERR_FINDING_USR, "send invitation: error getting invitor user preferences: ", err.Error())
		return
	}
	invitedUsr := a.findExistingUser(invite.Email, a.sl.TokenProvide())
	if invitedUsr != nil {
		invite.UserId = invitedUsr.UserID
		// let's get the invitee user preferences
		inviteeLanguage = a.getUserLanguage(invite.UserId, req, res)
	} else {
		// fallback to the creator language
		invitorPreferences := creatorMetaData.Preferences
		if invitorPreferences != nil && invitorPreferences.DisplayLanguageCode != "" {
			inviteeLanguage = invitorPreferences.DisplayLanguageCode
		}
	}

	if !a.addOrUpdateConfirmation(req.Context(), invite, res) {
		return
	}
	a.logAudit(req, "notif created")
	if creatorMetaData.Profile == nil {
		a.sendError(
			res,
			http.StatusInternalServerError,
			STATUS_ERR_FINDING_USR,
			"send invitation: error getting invitor user profile: ", invite.CreatorId,
		)
		return
	}
	fullName := creatorMetaData.Profile.FullName

	// if invitee is already a user (ie already has an account), he won't go to signup but login instead
	if invite.UserId == "" || content["WebPath"] == "" {
		content["WebPath"] = "login"
	}
	content["Invitor"] = fullName
	content["Email"] = invite.Email
	content["Duration"] = invite.GetReadableDuration()

	invite.Creator.Profile = &models.Profile{FullName: creatorMetaData.Profile.FullName}

	if a.createAndSendNotification(req, invite, content, inviteeLanguage) {
		a.logAudit(req, "invite sent")
	} else {
		a.logAudit(req, "invite failed to be sent")
		log.Print("Something happened generating an invite email")
		res.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	a.sendModelAsResWithStatus(res, invite, http.StatusOK)
}
