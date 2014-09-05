package api

import (
	"./../clients"
	"./../models"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"net/url"
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

func (a *Api) SetHandlers(prefix string, rtr *mux.Router) {

	rtr.HandleFunc("/status", a.GetStatus).Methods("GET")
	rtr.Handle("/emailtoaddress/{type}/{address}", varsHandler(a.EmailAddress)).Methods("GET", "POST")
}

func (h varsHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	h(res, req, vars)
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
