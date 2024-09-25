package api

import (
	"net/http"
	"regexp"

	tide "github.com/mdblp/tide-whisperer-v2/v3/model"

	log "github.com/sirupsen/logrus"

	"github.com/mdblp/shoreline/schema"

	"github.com/mdblp/go-common/v2/clients/status"
	"github.com/mdblp/hydrophone/models"
	"github.com/mdblp/hydrophone/utils/otp"
)

const (
	statusPinResetNoID     = "Required userid is missing"
	statusPinResetErr      = "Error sending PIN Reset"
	statusPinResetNoServer = "This API cannot be requested with server token"
	statusUserDoesNotExist = "This user does not exist"
	timeStep               = 1800 // time interval for the OTP = 30 minutes
	digits                 = 9    // nb digits for the OTP
	startTime              = 0    // start time for the OTP = EPOCH
)

// SendPinReset handles the pin reset http route
// @Summary Send an OTP for PIN Reset to a patient
// @Description  It sends an email that contains a time-based One-time password for PIN Reset
// @ID hydrophone-api-sendPinReset
// @Accept  json
// @Produce  json
// @Param userid path string true "user id"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "userId was not provided"
// @Failure 401 {object} status.Status "only authorized for token bearers"
// @Failure 403 {object} status.Status "only authorized for patients, not clinicians nor server token"
// @Failure 422 {object} status.Status "Error when sending the email (probably caused by the mailing service)"
// @Failure 500 {object} status.Status "Internal error while processing the request, detailed error returned in the body"
// @Router /send/pin-reset/{userid} [post]
// @security TidepoolAuth
func (a *Api) SendPinReset(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	// by default, language is EN. It will be overriden if preferred language is found later
	var userLanguage = "en"
	var newOTP *models.Confirmation
	var usrDetails *schema.UserData
	var err error

	if token := a.token(res, req); token == nil {
		return
	} else if token.IsServer {
		a.sendError(res, http.StatusForbidden, statusPinResetNoServer)
		return
	}

	userID := vars["userid"]
	if userID == "" {
		log.Printf("sendPinReset - %s", statusPinResetNoID)
		a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusBadRequest, statusPinResetNoID), http.StatusBadRequest)
		return
	}

	if usrDetails, err = a.sl.GetUser(userID, a.sl.TokenProvide()); err != nil {
		log.Printf("sendPinReset - %s err[%s]", STATUS_ERR_FINDING_USR, err.Error())
		a.sendModelAsResWithStatus(res, STATUS_ERR_FINDING_USR, http.StatusInternalServerError)
		return
	}

	if usrDetails == nil {
		log.Printf("sendPinReset - %s err[%s]", statusUserDoesNotExist, userID)
		a.sendModelAsResWithStatus(res, statusUserDoesNotExist, http.StatusBadRequest)
		return
	}

	if !usrDetails.HasRole("patient") {
		log.Printf("sendPinReset - Clinician/Caregiver account [%s] cannot receive PIN Reset message", usrDetails.UserID)
		a.sendModelAsResWithStatus(res, STATUS_ERR_CLINICAL_USR, http.StatusForbidden)
		return
	}
	sendOk, _, err := a.verifySendAttempts(req.Context(), models.TypePatientPinReset, usrDetails.UserID, "", "")
	if err != nil {
		log.Printf("sendPinReset - %s err[%s]", STATUS_ERR_COUNTING_CONF, err.Error())
		a.sendModelAsResWithStatus(res, STATUS_ERR_COUNTING_CONF, http.StatusInternalServerError)
		return
	}
	if !sendOk {
		log.Printf("sendPinReset - Too many attempts for pin reset on account [%v]", usrDetails.UserID)
		a.sendModelAsResWithStatus(res, STATUS_ERR_TOO_MANY_ATTEMPTS, http.StatusForbidden)
		return
	}
	// send PIN Reset OTP to patient
	// the secret for the TOTP is the concatenation of userID + IMEI + userID
	// first get the IMEI of the patient's handset
	var patientConfig *tide.PumpSettings

	if patientConfig, err = a.medicalData.GetSettings(req.Context(), usrDetails.UserID, getSessionToken(req), false); err != nil {
		a.sendError(res, http.StatusInternalServerError, statusPinResetErr, "error getting patient config: ", err.Error())
		return
	}

	if patientConfig.Device.Imei == "" {
		a.sendError(res, http.StatusInternalServerError, statusPinResetErr, "error getting patient config")
		return
	}

	// Prepare TOTP generator
	// here we want TOTP with time step of 30 minutes
	// So it will be valid between 1 second and 30 minutes
	// We assume the validator (i.e. the DBLG handset will check validation against the last 2 time steps)
	// So the patient will have, in reality, between 30 minutes and 1 hour to receive the TOTP and enter it in the handset
	var gen = otp.TOTPGenerator{
		TimeStep:  timeStep,
		StartTime: startTime,
		Secret:    userID + patientConfig.Device.Imei + userID,
		Digit:     digits,
	}

	var totp otp.TOTP = gen.Now()
	var re = regexp.MustCompile(`^(...)(...)(...)$`)

	userLanguage = a.getUserLanguage(userID, req, res)

	var templateName = models.TemplateNamePatientPinReset

	emailContent := map[string]string{
		"Email": usrDetails.Emails[0],
		"OTP":   re.ReplaceAllString(totp.OTP, `$1-$2-$3`),
	}

	// Create new confirmation with context data = totp
	newOTP, _ = models.NewConfirmationWithContext(models.TypePatientPinReset, templateName, usrDetails.UserID, totp)
	newOTP.Email = usrDetails.Emails[0]

	// Save confirmation in DB
	if a.addOrUpdateConfirmation(req.Context(), newOTP, res) {

		if a.createAndSendNotification(req, newOTP, emailContent, userLanguage) {
			log.Printf("sendPinReset - OTP sent for %s", userID)
			a.logAudit(req, "pin reset OTP sent")
			res.WriteHeader(http.StatusOK)
			res.Write([]byte("OK"))
		} else {
			log.Print("sendPinReset - Something happened generating a Pin Reset email")
			res.WriteHeader(http.StatusUnprocessableEntity)
		}
	}
}
