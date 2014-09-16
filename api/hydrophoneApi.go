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
	"github.com/tidepool-org/go-common/clients/shoreline"
)

type (
	Api struct {
		Store      clients.StoreClient
		notifier   clients.Notifier
		templates  models.TemplateConfig
		sl         shoreline.Client
		gatekeeper commonClients.Gatekeeper
		Config     Config
	}
	Config struct {
		ServerSecret string                 `json:"serverSecret"` //used for services
		Templates    *models.TemplateConfig `json:"emailTemplates"`
	}
	// this is the data structure for the invitation body
	InviteBody struct {
		Email       string                       `json:"email"`
		Permissions map[string]map[string]string `json:"permissions"`
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
	STATUS_NO_TOKEN                  = "No x-tidepool-session-token was found"
	STATUS_INVALID_TOKEN             = "The x-tidepool-session-token was invalid"
	STATUS_OK                        = "OK"
)

func InitApi(cfg Config, store clients.StoreClient, ntf clients.Notifier, sl shoreline.Client, gatekeeper commonClients.Gatekeeper) *Api {

	return &Api{
		Store:      store,
		Config:     cfg,
		notifier:   ntf,
		sl:         sl,
		gatekeeper: gatekeeper,
	}
}

func (a *Api) SetHandlers(prefix string, rtr *mux.Router) {

	rtr.HandleFunc("/status", a.GetStatus).Methods("GET")

	rtr.Handle("/emailtoaddress/{type}/{address}", varsHandler(a.EmailAddress)).Methods("GET", "POST")

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
	// GET /confirm/invite/:userid
	rtr.Handle("/signup/{userid}", varsHandler(a.Dummy)).Methods("GET")
	rtr.Handle("/invite/{userid}", varsHandler(a.Dummy)).Methods("GET")

	// GET /confirm/invitations/:userid
	rtr.Handle("/invitations/{userid}", varsHandler(a.Dummy)).Methods("GET")

	// PUT /confirm/dismiss/invite/:userid/:invited_by
	// PUT /confirm/dismiss/signup/:userid
	dismiss := rtr.PathPrefix("/dismiss").Subrouter()
	dismiss.Handle("/invite/{userid}/{invitedby}",
		varsHandler(a.DismissInvite)).Methods("PUT")
	dismiss.Handle("/signup/{userid}",
		varsHandler(a.Dummy)).Methods("PUT")

	// DELETE /confirm/:userid/invited/:invited_address
	// DELETE /confirm/signup/:userid
	rtr.Handle("/{userid}/invited/{invited_address}", varsHandler(a.Dummy)).Methods("DELETE")
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

func (a *Api) saveConfirmation(conf *models.Confirmation) {
	a.Store.UpsertConfirmation(conf)
}

func (a *Api) checkToken(res http.ResponseWriter, req *http.Request) bool {
	if token := req.Header.Get(TP_SESSION_TOKEN); token != "" {
		td := a.sl.CheckToken(token)

		if td == nil || td.IsServer == false {
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

func (a *Api) EmailAddress(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	if token := req.Header.Get(TP_SESSION_TOKEN); token != "" {

		if td := a.sl.CheckToken(token); td == nil {
			log.Println("bad token check ", td)
		}

		emailType := vars["type"]
		emailAddress, _ := url.QueryUnescape(vars["address"])

		if emailAddress != "" && emailType != "" {

			if confirmation, err := models.NewConfirmation(models.TypeCareteamInvite, emailAddress); err != nil {
				log.Println("Error creating template ", err)
				res.Write([]byte(STATUS_ERR_CREATING_CONFIRMATION))
				res.WriteHeader(http.StatusInternalServerError)
				return
			} else {
				//save it
				if err := a.Store.UpsertConfirmation(confirmation); err != nil {
					log.Println("Error saving the confirmation ", err)
					res.Write([]byte(STATUS_ERR_SAVING_CONFIRMATION))
					res.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					emailTemplate := models.NewTemplate()
					emailTemplate.Load(confirmation.Type, a.Config.Templates)
					emailTemplate.Parse(confirmation)

					if status, details := a.notifier.Send([]string{emailAddress}, "TODO", emailTemplate.GenerateContent); status != http.StatusOK {
						log.Printf("Issue sending email: Status [%d] Message [%s]", status, details)
						res.Write([]byte(STATUS_ERR_SENDING_EMAIL))
						res.WriteHeader(http.StatusInternalServerError)
						return
					} else {
						res.WriteHeader(http.StatusOK)
						return
					}
				}
			}
			return
		}
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	res.WriteHeader(http.StatusUnauthorized)
	return
}

func (a *Api) AcceptInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	if a.checkToken(res, req) {

		userid := vars["userid"]
		invitedby := vars["invitedby"]

		if userid == "" || invitedby == "" {
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

		if conf, err := a.Store.FindConfirmation(accept); err != nil {
			log.Println("Error finding the confirmation ", err)
			res.WriteHeader(http.StatusInternalServerError)
			res.Write([]byte(STATUS_ERR_FINDING_CONFIRMATION))
			return
		} else if conf != nil {
			conf.UpdateStatus(models.StatusCompleted)

			if err := a.Store.UpsertConfirmation(conf); err != nil {
				log.Println("Error saving the confirmation ", err)
				res.WriteHeader(http.StatusInternalServerError)
				res.Write([]byte(STATUS_ERR_SAVING_CONFIRMATION))
				return
			}
			log.Printf("id: '%s' invitor: '%s'", userid, invitedby)
			log.Printf("AcceptInvite() ignored request %s %s", req.Method, req.URL)
			res.WriteHeader(http.StatusOK)
			res.Write([]byte(STATUS_OK))
			return
		}
		res.WriteHeader(http.StatusNoContent)
		res.Write([]byte(STATUS_CONFIRMATION_NOT_FOUND))
		return

	}
	return
}

func (a *Api) DismissInvite(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if a.checkToken(res, req) {

		userid := vars["userid"]
		invitedby := vars["invitedby"]

		if userid == "" || invitedby == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		dismiss := &models.Confirmation{}
		if err := json.NewDecoder(req.Body).Decode(dismiss); err != nil {
			log.Printf("Err: %v\n", err)
			res.WriteHeader(http.StatusBadRequest)
			res.Write([]byte(STATUS_ERR_DECODING_CONFIRMATION))
			return
		}

		if dismiss.Key == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if conf, err := a.Store.FindConfirmation(dismiss); err != nil {
			log.Println("Error finding the confirmation ", err)
			res.WriteHeader(http.StatusInternalServerError)
			res.Write([]byte(STATUS_ERR_FINDING_CONFIRMATION))
			return
		} else if conf != nil {
			conf.UpdateStatus(models.StatusDeclined)

			if err := a.Store.UpsertConfirmation(conf); err != nil {
				log.Println("Error saving the confirmation ", err)
				res.WriteHeader(http.StatusInternalServerError)
				res.Write([]byte(STATUS_ERR_SAVING_CONFIRMATION))
				return
			}
		}
		log.Printf("id: '%s' invitor: '%s'", userid, invitedby)
		log.Printf("DismissInvite() ignored request %s %s", req.Method, req.URL)
		res.WriteHeader(http.StatusNoContent)
		return
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

		if ib.Email == "" || len(ib.Permissions) == 0 {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		invite, _ := models.NewConfirmationWithContext(models.TypeCareteamInvite, userid, req.Body)
		invite.ToEmail = ib.Email

		if err := a.Store.UpsertConfirmation(invite); err != nil {
			log.Println("Error saving the confirmation ", err)
			res.WriteHeader(http.StatusInternalServerError)
			res.Write([]byte(STATUS_ERR_SAVING_CONFIRMATION))
			return
		}

		log.Printf("id: '%s' em: '%s'  p: '%v'\n", userid, ib.Email, ib.Permissions)
		log.Printf("SendInvite() ignored request %s %s", req.Method, req.URL)
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(STATUS_OK))
		return
	}
	return
}
