package api

import (
	"log"
	"net/http"
	"net/url"

	"./../clients"
	"./../models"
	"github.com/gorilla/mux"
)

type (
	Api struct {
		Store     clients.StoreClient
		notifier  clients.Notifier
		templates models.EmailTemplate
		Config    Config
	}
	Config struct {
		ServerSecret string                `json:"serverSecret"` //used for services
		templates    *models.EmailTemplate `json:"emailTemplates"`
	}
	// this just makes it easier to bind a handler for the Handle function
	varsHandler func(http.ResponseWriter, *http.Request, map[string]string)
)

const (
	TP_SESSION_TOKEN         = "x-tidepool-session-token"
	STATUS_ERR_SENDING_EMAIL = "Error sending email"
)

func InitApi(cfg Config, store clients.StoreClient, notifier clients.Notifier) *Api {
	return &Api{
		Store:    store,
		Config:   cfg,
		notifier: notifier,
	}
}

// POST /confirm/send/signup/:userid
// POST /confirm/send/forgot/:useremail
// POST /confirm/send/invite/:userid

// POST /confirm/resend/signup/:userid

// PUT /confirm/accept/signup/:userid/:confirmationID
// PUT /confirm/accept/forgot/
// PUT /confirm/accept/invite/:userid/:invited_by

// GET /confirm/signup/:userid
// GET /confirm/invite/:userid

// GET /confirm/invitations/:userid

// PUT /confirm/dismiss/invite/:userid/:invited_by
// PUT /confirm/dismiss/signup/:userid

// DELETE /confirm/:userid/invited/:invited_address
// DELETE /confirm/signup/:userid

func (a *Api) SetHandlers(prefix string, rtr *mux.Router) {

	rtr.HandleFunc("/status", a.GetStatus).Methods("GET")
	rtr.Handle("/emailtoaddress/{type}/{address}", varsHandler(a.EmailAddress)).Methods("GET", "POST")

	send := rtr.PathPrefix("/send").Subrouter()
	send.Handle("/signup/{userid}", varsHandler(a.Dummy)).Methods("POST")
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
		log.Println(err)
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte(err.Error()))
		return
	}
	res.WriteHeader(http.StatusOK)
	return
}

func (a *Api) EmailAddress(res http.ResponseWriter, req *http.Request, vars map[string]string) {
	if td := req.Header.Get(TP_SESSION_TOKEN); td != "" {

		emailType := vars["type"]
		emailAddress, _ := url.QueryUnescape(vars["address"])

		if emailAddress != "" && emailType != "" {

			if status, err := a.notifier.Send([]string{emailAddress}, "TODO", "TODO"); err != nil {
				log.Println(err)
				res.Write([]byte(STATUS_ERR_SENDING_EMAIL))
				res.WriteHeader(http.StatusInternalServerError)
			} else {
				log.Println(status)
				res.WriteHeader(http.StatusOK)
			}
		}
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	res.WriteHeader(http.StatusUnauthorized)
	return
}
