package api

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	"./../clients"
	"./../models"

	"github.com/gorilla/mux"
	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/highwater"
	"github.com/tidepool-org/go-common/clients/shoreline"
)

type (
	Api struct {
		Store      clients.StoreClient
		notifier   clients.Notifier
		templates  models.TemplateConfig
		sl         shoreline.Client
		gatekeeper commonClients.Gatekeeper
		metrics    highwater.Client
		Config     Config
	}
	Config struct {
		ServerSecret string                 `json:"serverSecret"` //used for services
		Templates    *models.TemplateConfig `json:"emailTemplates"`
	}
	// this is the data structure for the invitation body
	InviteBody struct {
		Email       string                    `json:"email"`
		Permissions commonClients.Permissions `json:"permissions"`
	}
	inviteContent struct {
		CareteamName       string
		ViewAndUploadPerms bool
		ViewOnlyPerms      bool
	}
	// this just makes it easier to bind a handler for the Handle function
	varsHandler func(http.ResponseWriter, *http.Request, map[string]string)
)

const (
	TP_SESSION_TOKEN = "x-tidepool-session-token"
	//returned status messages
	STATUS_ERR_SENDING_EMAIL         = "Error sending email"
	STATUS_ERR_SAVING_CONFIRMATION   = "Error saving the confirmation"
	STATUS_ERR_CREATING_CONFIRMATION = "Error creating a confirmation"
	STATUS_ERR_FINDING_CONFIRMATION  = "Error finding the confirmation"
	STATUS_ERR_DECODING_CONFIRMATION = "Error decoding the confirmation"
	STATUS_CONFIRMATION_NOT_FOUND    = "No matching confirmation was found"
	STATUS_CONFIRMATION_CANCELED     = "Confirmation has been canceled"
	STATUS_NO_TOKEN                  = "No x-tidepool-session-token was found"
	STATUS_INVALID_TOKEN             = "The x-tidepool-session-token was invalid"
	STATUS_OK                        = "OK"
)

func InitApi(
	cfg Config,
	store clients.StoreClient,
	ntf clients.Notifier,
	sl shoreline.Client,
	gatekeeper commonClients.Gatekeeper,
	metrics highwater.Client) *Api {

	return &Api{
		Store:      store,
		Config:     cfg,
		notifier:   ntf,
		sl:         sl,
		gatekeeper: gatekeeper,
		metrics:    metrics,
	}
}

func (a *Api) SetHandlers(prefix string, rtr *mux.Router) {

	rtr.HandleFunc("/status", a.GetStatus).Methods("GET")

	// POST /confirm/send/signup/:userid
	// POST /confirm/send/forgot/:useremail
	// POST /confirm/send/invite/:userid
	send := rtr.PathPrefix("/send").Subrouter()
	send.Handle("/signup/{userid}", varsHandler(a.Dummy)).Methods("POST")
	send.Handle("/forgot/{useremail}", varsHandler(a.Dummy)).Methods("POST")
	send.Handle("/invite/{userid}", varsHandler(a.SendInvite)).Methods("POST")

	// POST /confirm/resend/signup/:userid
	rtr.Handle("/resend/signup/{userid}", varsHandler(a.Dummy)).Methods("POST")

	// PUT /confirm/accept/signup/:userid/:confirmationID
	// PUT /confirm/accept/forgot/
	// PUT /confirm/accept/invite/:userid/:invited_by
	accept := rtr.PathPrefix("/accept").Subrouter()
	accept.Handle("/signup/{userid}/{confirmationid}",
		varsHandler(a.Dummy)).Methods("PUT")
	accept.Handle("/forgot", varsHandler(a.Dummy)).Methods("PUT")
	accept.Handle("/invite/{userid}/{invitedby}",
		varsHandler(a.AcceptInvite)).Methods("PUT")

	// GET /confirm/signup/:userid
	// GET /confirm/invite/:useremail
	rtr.Handle("/signup/{userid}", varsHandler(a.Dummy)).Methods("GET")
	rtr.Handle("/invite/{useremail}", varsHandler(a.GetSentInvitations)).Methods("GET")

	// GET /confirm/invitations/:userid
	rtr.Handle("/invitations/{userid}", varsHandler(a.GetReceivedInvitations)).Methods("GET")

	// PUT /confirm/dismiss/invite/:userid/:invited_by
	// PUT /confirm/dismiss/signup/:userid
	dismiss := rtr.PathPrefix("/dismiss").Subrouter()
	dismiss.Handle("/invite/{userid}/{invitedby}",
		varsHandler(a.DismissInvite)).Methods("PUT")
	dismiss.Handle("/signup/{userid}",
		varsHandler(a.Dummy)).Methods("PUT")

	// PUT /confirm/:userid/invited/:invited_address
	// DELETE /confirm/signup/:userid
	rtr.Handle("/{userid}/invited/{invited_address}", varsHandler(a.CancelInvite)).Methods("PUT")
	rtr.Handle("/signup/{userid}", varsHandler(a.Dummy)).Methods("DELETE")
}

func (h varsHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	h(res, req, vars)
}

func (a *Api) Dummy(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	log.Printf("dummy() ignored request %s %s", req.Method, req.URL)
	res.WriteHeader(http.StatusOK)
}

func (a *Api) GetStatus(res http.ResponseWriter, req *http.Request) {
	if err := a.Store.Ping(); err != nil {
		log.Println("Error getting status", err)
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte(err.Error()))
		return
	}
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(STATUS_OK))
	return
}

//Save this confirmation or write and error if it all goes wrong
func (a *Api) addOrUpdateConfirmation(conf *models.Confirmation, res http.ResponseWriter) bool {
	if err := a.Store.UpsertConfirmation(conf); err != nil {
		log.Println("Error saving the confirmation ", err)
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte(STATUS_ERR_SAVING_CONFIRMATION))
		return false
	}
	return true
}

//Find this confirmation, write error if fails or write no-content if it doesn't exist
func (a *Api) findExistingConfirmation(conf *models.Confirmation, res http.ResponseWriter) *models.Confirmation {
	if found, err := a.Store.FindConfirmation(conf); err != nil {
		log.Println("Error finding the confirmation ", err)
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte(STATUS_ERR_FINDING_CONFIRMATION))
		return nil
	} else if found == nil {
		res.WriteHeader(http.StatusNoContent)
		res.Write([]byte(STATUS_CONFIRMATION_NOT_FOUND))
		return nil
	} else {
		return found
	}
}

//Do we already have a confirmation for address?
func (a *Api) hasExistingConfirmation(email string, status models.Status) bool {
	if found, err := a.Store.FindConfirmations(email, "", status); err != nil {
		log.Println("Error looking for existing confirmation ", err)
	} else if len(found) > 0 {
		return true
	}
	return false
}

//Find this confirmation, write error if fails or write no-content if it doesn't exist
func (a *Api) findConfirmations(userId, creatorId string, status models.Status, res http.ResponseWriter) []*models.Confirmation {
	if found, err := a.Store.FindConfirmations(userId, creatorId, status); err != nil {
		log.Println("Error finding confirmations ", err)
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte(STATUS_ERR_FINDING_CONFIRMATION))
		return nil
	} else if found == nil || len(found) == 0 {
		res.WriteHeader(http.StatusNoContent)
		res.Write([]byte(STATUS_CONFIRMATION_NOT_FOUND))
		return nil
	} else {
		return found
	}
}

//Generate a notification from the given confirmation,write the error if it fails
func (a *Api) createAndSendNotfication(conf *models.Confirmation, content interface{}, subject string) bool {

	emailTemplate := models.NewTemplate()
	emailTemplate.Load(conf.Type, a.Config.Templates)
	emailTemplate.Parse(content)

	if status, details := a.notifier.Send([]string{conf.Email}, subject, emailTemplate.GenerateContent); status != http.StatusOK {
		log.Printf("Issue sending email: Status [%d] Message [%s]", status, details)
		return false
	}
	return true
}

//find and validate the token
func (a *Api) checkToken(res http.ResponseWriter, req *http.Request) bool {
	if token := req.Header.Get(TP_SESSION_TOKEN); token != "" {
		td := a.sl.CheckToken(token)

		if td == nil {
			res.WriteHeader(http.StatusForbidden)
			res.Write([]byte(STATUS_INVALID_TOKEN))
			return false
		}
		//all good!
		return true
	}
	res.WriteHeader(http.StatusUnauthorized)
	res.Write([]byte(STATUS_NO_TOKEN))
	return false
}

//send metric
func (a *Api) logMetric(name string, req *http.Request) {
	token := req.Header.Get(TP_SESSION_TOKEN)
	emptyParams := make(map[string]string)
	a.metrics.PostThisUser(name, token, emptyParams)
	return
}

func sendModelAsResWithStatus(res http.ResponseWriter, model interface{}, statusCode int) {
	res.Header().Set("content-type", "application/json")
	res.WriteHeader(statusCode)

	if jsonDetails, err := json.Marshal(model); err != nil {
		log.Println(err)
	} else {
		res.Write(jsonDetails)
	}
	return
}

func (a *Api) GetReceivedInvitations(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if a.checkToken(res, req) {
		userid := vars["userid"]

		if userid == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}
		if invites := a.findConfirmations(userid, "", models.StatusPending, res); invites != nil {
			a.logMetric("get received invites", req)
			sendModelAsResWithStatus(res, invites, http.StatusOK)
			return
		}
	}
	return
}

func (a *Api) GetSentInvitations(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if a.checkToken(res, req) {

		userEmail, _ := url.QueryUnescape(vars["useremail"])

		if userEmail == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}
		if invitations := a.findConfirmations("", userEmail, models.StatusPending, res); invitations != nil {
			a.logMetric("get sent invites", req)
			sendModelAsResWithStatus(res, invitations, http.StatusOK)
			return
		}
	}
	return
}

func (a *Api) AcceptInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	if a.checkToken(res, req) {

		userId := vars["userid"]
		invitorId := vars["invitedby"]

		if userId == "" || invitorId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		accept := &models.Confirmation{}
		if err := json.NewDecoder(req.Body).Decode(accept); err != nil {
			log.Printf("Err: %v\n", err)
			res.WriteHeader(http.StatusBadRequest)
			res.Write([]byte(STATUS_ERR_DECODING_CONFIRMATION))
			return
		}

		if accept.Key == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if conf := a.findExistingConfirmation(accept, res); conf != nil {

			//New set the permissions for the invite
			var permissions commonClients.Permissions
			conf.DecodeContext(&permissions)

			if setPerms, err := a.gatekeeper.SetPermissions(userId, invitorId, permissions); err != nil {
				log.Println("Error setting permissions in AcceptInvite ", err)
				res.WriteHeader(http.StatusInternalServerError)
				res.Write([]byte(STATUS_ERR_DECODING_CONFIRMATION))
				return
			} else {
				log.Printf("Permissions were set as [%v] after an invite was accepted", setPerms)
				//we know the user now
				conf.UserId = userId

				conf.UpdateStatus(models.StatusCompleted)
				if a.addOrUpdateConfirmation(conf, res) {
					a.logMetric("acceptinvite", req)
					res.WriteHeader(http.StatusOK)
					res.Write([]byte(STATUS_OK))
					return
				}
			}
		}
	}
	return
}

func (a *Api) CancelInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if a.checkToken(res, req) {

		invitorId := vars["userid"]
		email := vars["invited_address"]

		if invitorId == "" || email == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		invite := &models.Confirmation{
			Email:     email,
			CreatorId: invitorId,
			Type:      models.TypeCareteamInvite,
		}

		if conf := a.findExistingConfirmation(invite, res); conf != nil {
			//cancel the invite
			conf.UpdateStatus(models.StatusCanceled)

			if a.addOrUpdateConfirmation(conf, res) {
				a.logMetric("canceled invite", req)
				res.WriteHeader(http.StatusOK)
				res.Write([]byte(STATUS_CONFIRMATION_CANCELED))
				return
			}
		}
		res.WriteHeader(http.StatusNoContent)
		res.Write([]byte(STATUS_CONFIRMATION_NOT_FOUND))
		return
	}
	return
}

func (a *Api) DismissInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if a.checkToken(res, req) {

		userId := vars["userid"]
		invitorId := vars["invitedby"]

		if userId == "" || invitorId == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		dismiss := &models.Confirmation{}
		if err := json.NewDecoder(req.Body).Decode(dismiss); err != nil {
			log.Printf("Error decoding invite to dismiss [%v]", err)
			res.WriteHeader(http.StatusBadRequest)
			res.Write([]byte(STATUS_ERR_DECODING_CONFIRMATION))
			return
		}

		if dismiss.Key == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if conf := a.findExistingConfirmation(dismiss, res); conf != nil {

			conf.UpdateStatus(models.StatusDeclined)

			if a.addOrUpdateConfirmation(conf, res) {
				a.logMetric("dismissinvite", req)
				res.WriteHeader(http.StatusNoContent)
				res.Write([]byte(STATUS_OK))
				return
			}
		}
	}
	return
}

func (a *Api) SendInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if a.checkToken(res, req) {

		userid := vars["userid"]

		defer req.Body.Close()
		var ib = &InviteBody{}
		if err := json.NewDecoder(req.Body).Decode(ib); err != nil {
			log.Printf("Err: %v\n", err)
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if ib.Email == "" || ib.Permissions == nil {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		invite, _ := models.NewConfirmationWithContext(models.TypeCareteamInvite, userid, ib.Permissions)

		invite.Email = ib.Email

		if a.hasExistingConfirmation(invite.Email, models.StatusPending) {
			log.Printf("There is already an existing invite [%v]", invite)
			res.WriteHeader(http.StatusConflict)
			return
		}

		if a.addOrUpdateConfirmation(invite, res) {
			a.logMetric("invite created", req)

			viewOnly := ib.Permissions["upload"] == ""

			inviteContent := &inviteContent{CareteamName: "Todo", ViewAndUploadPerms: viewOnly == false, ViewOnlyPerms: viewOnly}

			if a.createAndSendNotfication(invite, inviteContent, "Invite to join my careteam") {
				a.logMetric("invite sent", req)
			}
			sendModelAsResWithStatus(res, invite, http.StatusOK)
			return
		}
	}
	return
}
