package api

import (
	"log"
	"net/http"

	"./../models"
)

const (
	STATUS_NO_EMAIL_MATCH = ""
	STATUS_RESET_SENT     = ""
	STATUS_RESET_ACCEPTED = ""
)

type (
	//Content used to generate the reset email
	resetEmailContent struct {
		Key   string
		Email string
	}
	//reset details reseting a users password
	resetBody struct {
		Key      string `json:"key"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
)

//Send a password reset confirmation
//
// status: 200 STATUS_RESET_SENT
// status: 404 STATUS_NO_EMAIL_MATCH - no match is made on the email
// status: 400 no email given
func (a *Api) passwordReset(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if a.checkToken(res, req) {
		email := vars["useremail"]
		if email == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		resetCnf, _ := models.NewConfirmation(models.TypePasswordReset, "")
		resetCnf.Email = email

		//already in the group?
		if resetUsr := a.findExistingUser(resetCnf.Email, token); resetUsr != nil {
			resetCnf.UserId = resetUsr.UserID
		}

		if a.addOrUpdateConfirmation(resetCnf, res) {
			a.logMetric("reset confirmation created", req)

			emailContent := &resetEmailContent{
				Key:   resetCnf.Key,
				Email: resetCnf.Email,
			}

			if a.createAndSendNotfication(invite, emailContent) {
				a.logMetric("reset confirmation sent", req)
			}
		}

		a.sendModelAsResWithStatus(res, resetCnf, http.StatusOK)
		return
	}
	return
}

//Accept the password change
//
// status: 200 STATUS_RESET_ACCEPTED
/*
```
PUT /confirm/accept/forgot/

body {
  "key": "confirmkey"
  "email": "address_on_the_account"
  "password": "new_password"
}
```

This call will be invoked by the lost password screen with the key that was included in the URL of the lost password screen. For additional safety, the user will be required to manually enter the email address on the account as part of the UI, and also to enter a new password which will replace the password on the account.

If this call is completed without error, the lost password request is marked as accepted. Otherwise, the lost password request remains active until it expires.

*/
func (a *Api) acceptPassword(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if a.checkToken(res, req) {

		defer req.Body.Close()
		var rb = &resetBody{}
		if err := json.NewDecoder(req.Body).Decode(rb); err != nil {
			log.Printf("acceptPassword: error decoding reset details %v\n", err)
			statusErr := &status.StatusError{status.NewStatus(http.StatusBadRequest, STATUS_ERR_DECODING_CONFIRMATION)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusBadRequest)
			return
		}

		res.WriteHeader(http.StatusNotImplemented)
		return
	}
	return
}
