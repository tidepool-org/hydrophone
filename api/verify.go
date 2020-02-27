package api

import (
	"log"
	"net/http"

	"github.com/tidepool-org/go-common/clients/status"
)

const (
	STATUS_VERIFY_NO_ID               = "Required userid is missing"
	STATUS_VERIFY_USER_DOES_NOT_EXIST = "User does not exist"
)

// @Summary Sanity check the service configuration
// @Description  Send a test email to prove all configuration is in place for sending emails
// @ID hydrophone-api-sanityCheck
// @Accept  json
// @Produce  json
// @Param userid path string true "user id"
// @Success 200 {string} string "OK"
// @Failure 500 {object} status.Status "Error while generating/sending the email"
// @Failure 400 {object} status.Status "userid was not provided or incorrect "
// @Router /sanity_check/{userid} [post]
// @security TidepoolAuth
func (a *Api) sendSanityCheckEmail(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	log.Printf("Sanity check email route")

	subject := "Sanity Check Email"
	body := "This is an automatic email sent from Hydrophone service to prove all configuration is in place for sending emails"
	recipient := ""

	if token := a.token(res, req); token != nil {

		userID := vars["userid"]
		if userID == "" {
			log.Printf("sanityCheck %s", STATUS_VERIFY_NO_ID)
			a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusBadRequest, STATUS_VERIFY_NO_ID), http.StatusBadRequest)
			return
		}

		// To ensure this route is not used for spamming, we ensure session token and {userid} param match an actual user
		// In case the user exists, we get details about him
		if usrDetails, err := a.sl.GetUser(userID, a.sl.TokenProvide()); err != nil {
			log.Printf("sanityCheck %s err[%s]", STATUS_ERR_FINDING_USER, err.Error())
			a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusInternalServerError, STATUS_ERR_FINDING_USER), http.StatusInternalServerError)
			return
		} else {

			if usrDetails == nil {
				log.Printf("sanityCheck %s [%s]", STATUS_VERIFY_USER_DOES_NOT_EXIST, userID)
				a.sendModelAsResWithStatus(res, status.NewStatus(http.StatusBadRequest, STATUS_VERIFY_USER_DOES_NOT_EXIST), http.StatusBadRequest)
				return
			}

			// The test email recipient is the user found existing
			recipient = usrDetails.Emails[0]

			// Here, we assume the email address found for the user is valid

			// Try sending
			if status, details := a.notifier.Send([]string{recipient}, subject, body); status != http.StatusOK {
				log.Printf("Issue sending sanity check email: Status [%d] Message [%s]", status, details)
				res.WriteHeader(http.StatusInternalServerError)
				return
			}

			log.Printf("Success: sanity check email successfully sent to [%s]", recipient)

			res.WriteHeader(http.StatusOK)
			res.Write([]byte(STATUS_OK))
			return
		}
	}
}
