package api

import (
	"log"
	"net/http"
	"regexp"

	otp "github.com/tidepool-org/hydrophone/utils/otp"

	"github.com/tidepool-org/go-common/clients/portal"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/hydrophone/models"
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
	var usrDetails *shoreline.UserData
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

	if usrDetails.IsClinic() {
		log.Printf("sendPinReset - Clinician account [%s] cannot receive PIN Reset message", usrDetails.UserID)
		a.sendModelAsResWithStatus(res, STATUS_ERR_CLINICAL_USR, http.StatusForbidden)
		return
	}

	// send PIN Reset OTP to patient
	// the secret for the TOTP is the concatenation of userID + IMEI + userID
	// first get the IMEI of the patient's handset
	var patientConfig *portal.PatientConfig

	// actual token value is needed to call portal client: we know it does exist and is a user one
	if patientConfig, err = a.portal.GetPatientConfig(req.Header.Get(TP_SESSION_TOKEN)); err != nil {
		a.sendError(res, http.StatusInternalServerError, statusPinResetErr, "error getting patient config: ", err.Error())
		return
	}

	if patientConfig.Device.IMEI == "" {
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
		Secret:    userID + patientConfig.Device.IMEI + userID,
		Digit:     digits,
	}

	var totp otp.TOTP = gen.Now()
	var re = regexp.MustCompile(`^(...)(...)(...)$`)

	// let's get the user preferences
	userPreferences := &models.Preferences{}
	if err := a.seagull.GetCollection(userID, "preferences", a.sl.TokenProvide(), userPreferences); err != nil {
		a.sendError(res, http.StatusInternalServerError, statusPinResetErr, "error getting user preferences: ", err.Error())
		return
	}

	// does the user have a preferred language?
	if userPreferences.DisplayLanguage != "" {
		userLanguage = userPreferences.DisplayLanguage
	}

	var templateName = models.TemplateNamePatientPinReset

	emailContent := map[string]interface{}{
		"Email": usrDetails.Emails[0],
		"OTP":   re.ReplaceAllString(totp.OTP, `$1-$2-$3`),
	}

	// Create new confirmation with context data = totp
	newOTP, _ = models.NewConfirmationWithContext(models.TypePatientPinReset, templateName, usrDetails.UserID, totp)
	newOTP.Email = usrDetails.Emails[0]

	// Save confirmation in DB
	if a.addOrUpdateConfirmation(newOTP, res) {

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
