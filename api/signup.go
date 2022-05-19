package api

import (
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/mdblp/go-common/clients/status"
	"github.com/mdblp/hydrophone/models"
)

// @Summary Patient account creation informative email
// @Description  Send an informative email to the patient to notify the account was successfully created
// @ID hydrophone-api-sendSignUpInformation
// @Accept  json
// @Produce  json
// @Param userid path string true "user id"
// @Success 200 {string} string "OK"
// @Failure 400 {object} status.Status "userId was not provided, return:\"Required userid is missing\" "
// @Failure 403 {object} status.Status "Operation forbiden for this account, return detailed error"
// @Failure 422 {object} status.Status "Error when sending the email"
// @Failure 500 {object} status.Status "Error finding the user, message returned:\"Error finding the user\" "
// @Router /send/inform/{userid} [post]
func (a *Api) sendSignUpInformation(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	var signerLanguage string
	var newSignUp *models.Confirmation

	userID := vars["userid"]
	if userID == "" {
		log.Printf("sendSignUp %s", STATUS_SIGNUP_NO_ID)
		a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusBadRequest, STATUS_SIGNUP_NO_ID), http.StatusBadRequest)
		return
	}
	if usrDetails, err := a.sl.GetUser(userID, a.sl.TokenProvide()); err != nil {
		log.Printf("sendSignUp %s err[%s]", STATUS_ERR_FINDING_USER, err.Error())
		a.sendModelAsResWithStatus(res, status.StatusError{status.NewStatus(http.StatusInternalServerError, STATUS_ERR_FINDING_USER)}, http.StatusInternalServerError)
		return
	} else {
		if !usrDetails.HasRole("patient") {
			log.Printf("Clinician/Caregiver account [%s] cannot receive information message", usrDetails.UserID)
			a.sendModelAsResWithStatus(res, STATUS_ERR_CLINICAL_USR, http.StatusForbidden)
			return
		}
		if !usrDetails.EmailVerified {
			log.Printf("User [%s] is not yet verified", usrDetails.UserID)
			a.sendModelAsResWithStatus(res, STATUS_ERR_FINDING_VALIDATION, http.StatusForbidden)
			return
		} else {
			// send information message to patient
			var templateName = models.TemplateNamePatientInformation

			emailContent := map[string]string{
				"Email": usrDetails.Emails[0],
			}

			newSignUp, _ = models.NewConfirmation(models.TypeInformation, templateName, usrDetails.UserID)
			newSignUp.Email = usrDetails.Emails[0]

			// this one may not work if the language is not set by the caller
			if signerLanguage = GetBrowserPreferredLanguage(req); signerLanguage == "" {
				signerLanguage = "en"
			}

			if a.createAndSendNotification(req, newSignUp, emailContent, signerLanguage) {
				log.Printf("signup information sent for %s", userID)
				a.logAudit(req, "signup information sent")
				res.WriteHeader(http.StatusOK)
				return
			} else {
				a.logAudit(req, "signup confirmation failed to be sent")
				log.Print("Something happened generating a signup email")
				res.WriteHeader(http.StatusUnprocessableEntity)
			}
		}
	}

}
