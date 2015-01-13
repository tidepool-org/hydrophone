package api

import (
	"encoding/json"
	"log"
	"net/http"

	"./../clients"
	"./../models"

	"github.com/gorilla/mux"
	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/highwater"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
)

type (
	Api struct {
		Store      clients.StoreClient
		notifier   clients.Notifier
		templates  models.TemplateConfig
		sl         shoreline.Client
		gatekeeper commonClients.Gatekeeper
		seagull    commonClients.Seagull
		metrics    highwater.Client
		Config     Config
	}
	Config struct {
		ServerSecret      string                 `json:"serverSecret"` //used for services
		Templates         *models.TemplateConfig `json:"emailTemplates"`
		InviteTimeoutDays int                    `json:"inviteTimeoutDays"`
		ResetTimeoutDays  int                    `json:"resetTimeoutDays"`
	}
	profile struct {
		FullName string
	}
	group struct {
		Members []string
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
	STATUS_ERR_FINDING_PREVIEW       = "Error finding the invite preview"
	STATUS_NOT_FOUND                 = "Nothing found"
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
	metrics highwater.Client,
	seagull commonClients.Seagull,
) *Api {
	return &Api{
		Store:      store,
		Config:     cfg,
		notifier:   ntf,
		sl:         sl,
		gatekeeper: gatekeeper,
		metrics:    metrics,
		seagull:    seagull,
	}
}

func (a *Api) SetHandlers(prefix string, rtr *mux.Router) {

	rtr.HandleFunc("/status", a.GetStatus).Methods("GET")

	// POST /confirm/send/signup/:userid
	// POST /confirm/send/forgot/:useremail
	// POST /confirm/send/invite/:userid
	send := rtr.PathPrefix("/send").Subrouter()
	send.Handle("/signup/{userid}", varsHandler(a.Dummy)).Methods("POST")
	send.Handle("/forgot/{useremail}", varsHandler(a.passwordReset)).Methods("POST")
	send.Handle("/invite/{userid}", varsHandler(a.SendInvite)).Methods("POST")

	// POST /confirm/resend/signup/:userid
	rtr.Handle("/resend/signup/{userid}", varsHandler(a.Dummy)).Methods("POST")

	// PUT /confirm/accept/signup/:userid/:confirmationID
	// PUT /confirm/accept/forgot/
	// PUT /confirm/accept/invite/:userid/:invited_by
	accept := rtr.PathPrefix("/accept").Subrouter()
	accept.Handle("/signup/{userid}/{confirmationid}", varsHandler(a.Dummy)).Methods("PUT")
	accept.Handle("/forgot", varsHandler(a.acceptPassword)).Methods("PUT")
	accept.Handle("/invite/{userid}/{invitedby}", varsHandler(a.AcceptInvite)).Methods("PUT")

	// GET /confirm/signup/:userid
	// GET /confirm/invite/:userid
	rtr.Handle("/signup/{userid}", varsHandler(a.Dummy)).Methods("GET")
	rtr.Handle("/invite/{userid}", varsHandler(a.GetSentInvitations)).Methods("GET")

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
		log.Printf("Error getting status [%v]", err)
		statusErr := &status.StatusError{status.NewStatus(http.StatusInternalServerError, err.Error())}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(STATUS_OK))
	return
}

//Save this confirmation or
//write an error if it all goes wrong
func (a *Api) addOrUpdateConfirmation(conf *models.Confirmation, res http.ResponseWriter) bool {
	if err := a.Store.UpsertConfirmation(conf); err != nil {
		log.Printf("Error saving the confirmation [%v]", err)
		statusErr := &status.StatusError{status.NewStatus(http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusInternalServerError)
		return false
	}
	return true
}

//Find this confirmation
//write error if it fails
func (a *Api) findExistingConfirmation(conf *models.Confirmation, res http.ResponseWriter) (*models.Confirmation, error) {
	if found, err := a.Store.FindConfirmation(conf); err != nil {
		log.Printf("findExistingConfirmation: [%v]", err)
		statusErr := &status.StatusError{status.NewStatus(http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION)}
		return nil, statusErr
	} else {
		return found, nil
	}
}

//Find these confirmations
//write error if fails or write no-content if it doesn't exist
func (a *Api) checkFoundConfirmations(res http.ResponseWriter, results []*models.Confirmation, err error) []*models.Confirmation {
	if err != nil {
		log.Println("Error finding confirmations ", err)
		statusErr := &status.StatusError{status.NewStatus(http.StatusInternalServerError, STATUS_ERR_FINDING_CONFIRMATION)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusInternalServerError)
		return nil
	} else if results == nil || len(results) == 0 {
		statusErr := &status.StatusError{status.NewStatus(http.StatusNotFound, STATUS_NOT_FOUND)}
		log.Printf("No confirmations were found [%s]", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
		return nil
	} else {
		return results
	}
}

//Generate a notification from the given confirmation,write the error if it fails
func (a *Api) createAndSendNotfication(conf *models.Confirmation, content interface{}) bool {

	emailTemplate := models.NewTemplate()
	emailTemplate.Load(conf.Type, a.Config.Templates)
	emailTemplate.Parse(content)

	if status, details := a.notifier.Send([]string{conf.Email}, emailTemplate.Subject, emailTemplate.BodyContent); status != http.StatusOK {
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
			statusErr := &status.StatusError{status.NewStatus(http.StatusForbidden, STATUS_INVALID_TOKEN)}
			a.sendModelAsResWithStatus(res, statusErr, http.StatusForbidden)
			return false
		}
		//all good!
		return true
	}
	statusErr := &status.StatusError{status.NewStatus(http.StatusUnauthorized, STATUS_NO_TOKEN)}
	a.sendModelAsResWithStatus(res, statusErr, http.StatusUnauthorized)
	return false
}

//send metric
func (a *Api) logMetric(name string, req *http.Request) {
	token := req.Header.Get(TP_SESSION_TOKEN)
	emptyParams := make(map[string]string)
	a.metrics.PostThisUser(name, token, emptyParams)
	return
}

//send metric
func (a *Api) logMetricAsServer(name string) {
	token := a.sl.TokenProvide()
	emptyParams := make(map[string]string)
	a.metrics.PostServer(name, token, emptyParams)
	return
}

//Find existing user based on the given indentifier
//The indentifier could be either an id or email address
func (a *Api) findExistingUser(indentifier, token string) *shoreline.UserData {
	if usr, err := a.sl.GetUser(indentifier, token); err != nil {
		log.Printf("Error trying to get existing users details [%v]", err)
		return nil
	} else {
		return usr
	}
}

//Makesure we have set the userId on these confirmations
func (a *Api) ensureIdSet(userId string, confirmations []*models.Confirmation) {

	if len(confirmations) < 1 {
		return
	}
	for i := range confirmations {
		//set the userid if not set already
		if confirmations[i].UserId == "" {
			log.Println("UserId wasn't set for invite so setting it")
			confirmations[i].UserId = userId
			a.Store.UpsertConfirmation(confirmations[i])
		}
		return
	}
}

func (a *Api) sendModelAsResWithStatus(res http.ResponseWriter, model interface{}, statusCode int) {
	if jsonDetails, err := json.Marshal(model); err != nil {
		log.Printf("Error trying to send [%v]", model)
		http.Error(res, "Error marshaling data for response", http.StatusInternalServerError)
	} else {
		res.Header().Set("content-type", "application/json")
		res.WriteHeader(statusCode)
		res.Write(jsonDetails)
	}
	return
}
