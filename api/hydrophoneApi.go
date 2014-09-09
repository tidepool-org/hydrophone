package api

import (
	"./../clients"
	"./../models"
	"github.com/gorilla/mux"
	//"github.com/tidepool-org/go-common/clients/shoreline"
	"log"
	"net/http"
	"net/url"
)

type (
	Api struct {
		Store     clients.StoreClient
		notifier  clients.Notifier
		templates models.EmailTemplate
		//shoreline *shoreline.ShorelineClient
		Config Config
	}
	Config struct {
		ServerSecret string                `json:"serverSecret"` //used for services
		Templates    *models.EmailTemplate `json:"emailTemplates"`
	}
	varsHandler func(http.ResponseWriter, *http.Request, map[string]string)
)

const (
	TP_SESSION_TOKEN         = "x-tidepool-session-token"
	STATUS_ERR_SENDING_EMAIL = "Error sending email"
	STATUS_OK                = "OK"
)

func InitApi(cfg Config, store clients.StoreClient, ntf clients.Notifier) *Api {

	return &Api{
		Store:    store,
		Config:   cfg,
		notifier: ntf,
		//shoreline: sl,
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
		log.Println("Error getting status", err)
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte(err.Error()))
		return
	}
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(STATUS_OK))
	return
}

func (a *Api) EmailAddress(res http.ResponseWriter, req *http.Request, vars map[string]string) {

	if token := req.Header.Get(TP_SESSION_TOKEN); token != "" {

		/* TODO: the actual token check once we have mocks in place
		if td := a.shoreline.CheckToken(token); td == nil {
			log.Println("bad token check ", td)
		}*/

		emailType := vars["type"]
		emailAddress, _ := url.QueryUnescape(vars["address"])

		if emailAddress != "" && emailType != "" {

			notification, err := models.NewEmailNotification(emailType, a.Config.Templates, emailAddress)

			if err != nil {
				log.Println("Error creating template ", err)
				res.Write([]byte(STATUS_ERR_SENDING_EMAIL))
				res.WriteHeader(http.StatusInternalServerError)
				return
			} else {
				if status, details := a.notifier.Send([]string{emailAddress}, "TODO", notification.Content); status != http.StatusOK {
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
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	res.WriteHeader(http.StatusUnauthorized)
	return
}
