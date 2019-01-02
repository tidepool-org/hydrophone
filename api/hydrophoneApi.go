package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"runtime"
	"strings"

	"github.com/gorilla/mux"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/highwater"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/hydrophone/clients"
	"github.com/tidepool-org/hydrophone/models"
)

type (
	Api struct {
		Store          clients.StoreClient
		notifier       clients.Notifier
		templates      models.Templates
		sl             shoreline.Client
		gatekeeper     commonClients.Gatekeeper
		seagull        commonClients.Seagull
		metrics        highwater.Client
		Config         Config
		LanguageBundle *i18n.Bundle
	}
	Config struct {
		ServerSecret      string `json:"serverSecret"` //used for services
		WebURL            string `json:"webUrl"`
		AssetURL          string `json:"assetUrl"`
		I18nTemplatesPath string `json:"i18nTemplatesPath"`
	}

	group struct {
		Members []string
	}
	// this just makes it easier to bind a handler for the Handle function
	varsHandler func(http.ResponseWriter, *http.Request, map[string]string)
)

const (
	TP_SESSION_TOKEN = "x-tidepool-session-token"

	//returned error messages
	STATUS_ERR_SENDING_EMAIL         = "Error sending email"
	STATUS_ERR_SAVING_CONFIRMATION   = "Error saving the confirmation"
	STATUS_ERR_CREATING_CONFIRMATION = "Error creating a confirmation"
	STATUS_ERR_FINDING_CONFIRMATION  = "Error finding the confirmation"
	STATUS_ERR_FINDING_USER          = "Error finding the user"
	STATUS_ERR_DECODING_CONFIRMATION = "Error decoding the confirmation"
	STATUS_ERR_FINDING_PREVIEW       = "Error finding the invite preview"

	//returned status messages
	STATUS_NOT_FOUND     = "Nothing found"
	STATUS_NO_TOKEN      = "No x-tidepool-session-token was found"
	STATUS_INVALID_TOKEN = "The x-tidepool-session-token was invalid"
	STATUS_UNAUTHORIZED  = "Not authorized for requested operation"
	STATUS_OK            = "OK"
)

func InitApi(
	cfg Config,
	store clients.StoreClient,
	ntf clients.Notifier,
	sl shoreline.Client,
	gatekeeper commonClients.Gatekeeper,
	metrics highwater.Client,
	seagull commonClients.Seagull,
	templates models.Templates,
) *Api {
	return &Api{
		Store:          store,
		Config:         cfg,
		notifier:       ntf,
		sl:             sl,
		gatekeeper:     gatekeeper,
		metrics:        metrics,
		seagull:        seagull,
		templates:      templates,
		LanguageBundle: nil,
	}
}

// InitApiI18n initializes both the API and the i18n artefacts
func InitApiWithI18n(
	cfg Config,
	store clients.StoreClient,
	ntf clients.Notifier,
	sl shoreline.Client,
	gatekeeper commonClients.Gatekeeper,
	metrics highwater.Client,
	seagull commonClients.Seagull,
	templates models.Templates,
) *Api {
	var theAPI *Api
	theAPI = &Api{
		Store:          store,
		Config:         cfg,
		notifier:       ntf,
		sl:             sl,
		gatekeeper:     gatekeeper,
		metrics:        metrics,
		seagull:        seagull,
		templates:      templates,
		LanguageBundle: nil,
	}
	theAPI.InitI18n(cfg.I18nTemplatesPath)
	return theAPI
}

func (a *Api) SetHandlers(prefix string, rtr *mux.Router) {

	rtr.HandleFunc("/status", a.GetStatus).Methods("GET")

	// POST /confirm/send/signup/:userid
	// POST /confirm/send/forgot/:useremail
	// POST /confirm/send/invite/:userid
	send := rtr.PathPrefix("/send").Subrouter()
	send.Handle("/signup/{userid}", varsHandler(a.sendSignUp)).Methods("POST")
	send.Handle("/forgot/{useremail}", varsHandler(a.passwordReset)).Methods("POST")
	send.Handle("/invite/{userid}", varsHandler(a.SendInvite)).Methods("POST")

	// POST /confirm/resend/signup/:useremail
	rtr.Handle("/resend/signup/{useremail}", varsHandler(a.resendSignUp)).Methods("POST")

	// PUT /confirm/accept/signup/:confirmationID
	// PUT /confirm/accept/forgot/
	// PUT /confirm/accept/invite/:userid/:invited_by
	accept := rtr.PathPrefix("/accept").Subrouter()
	accept.Handle("/signup/{confirmationid}", varsHandler(a.acceptSignUp)).Methods("PUT")
	accept.Handle("/forgot", varsHandler(a.acceptPassword)).Methods("PUT")
	accept.Handle("/invite/{userid}/{invitedby}", varsHandler(a.AcceptInvite)).Methods("PUT")

	// GET /confirm/signup/:userid
	// GET /confirm/invite/:userid
	rtr.Handle("/signup/{userid}", varsHandler(a.getSignUp)).Methods("GET")
	rtr.Handle("/invite/{userid}", varsHandler(a.GetSentInvitations)).Methods("GET")

	// GET /confirm/invitations/:userid
	rtr.Handle("/invitations/{userid}", varsHandler(a.GetReceivedInvitations)).Methods("GET")

	// PUT /confirm/dismiss/invite/:userid/:invited_by
	// PUT /confirm/dismiss/signup/:userid
	dismiss := rtr.PathPrefix("/dismiss").Subrouter()
	dismiss.Handle("/invite/{userid}/{invitedby}",
		varsHandler(a.DismissInvite)).Methods("PUT")
	dismiss.Handle("/signup/{userid}",
		varsHandler(a.dismissSignUp)).Methods("PUT")

	// PUT /confirm/:userid/invited/:invited_address
	// PUT /confirm/signup/:userid
	rtr.Handle("/{userid}/invited/{invited_address}", varsHandler(a.CancelInvite)).Methods("PUT")
	rtr.Handle("/signup/{userid}", varsHandler(a.cancelSignUp)).Methods("PUT")
}

func (h varsHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	h(res, req, vars)
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

//Find this confirmation
//write error if it fails
func (a *Api) addProfile(conf *models.Confirmation) error {
	if conf.CreatorId != "" {
		if err := a.seagull.GetCollection(conf.CreatorId, "profile", a.sl.TokenProvide(), &conf.Creator.Profile); err != nil {
			log.Printf("error getting the creators profile [%v] ", err)
			return err
		}

		conf.Creator.UserId = conf.CreatorId
	}
	return nil
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
		log.Println("No confirmations were found ", statusErr.Error())
		a.sendModelAsResWithStatus(res, statusErr, http.StatusNotFound)
		return nil
	} else {
		for i := range results {
			if err = a.addProfile(results[i]); err != nil {
				//report and move on
				log.Println("Error getting profile", err.Error())
			}
		}
		return results
	}
}

//Generate a notification from the given confirmation,write the error if it fails
func (a *Api) createAndSendNotification(conf *models.Confirmation, content map[string]interface{}, lang string) bool {

	log.Printf("sending notification with template %s to %s with language %s", conf.TemplateName, conf.Email, lang)

	// Get the template name based on the requested communication type
	templateName := conf.TemplateName
	if templateName == models.TemplateNameUndefined {
		switch conf.Type {
		case models.TypePasswordReset:
			templateName = models.TemplateNamePasswordReset
		case models.TypeCareteamInvite:
			templateName = models.TemplateNameCareteamInvite
		case models.TypeSignUp:
			templateName = models.TemplateNameSignup
		case models.TypeNoAccount:
			templateName = models.TemplateNameNoAccount
		default:
			log.Printf("Unknown confirmation type %s", conf.Type)
			return false
		}
	}

	// Content collection is here to replace placeholders in template body/content
	content["WebURL"] = a.Config.WebURL
	content["AssetURL"] = a.Config.AssetURL

	// Retrieve the template from all the preloaded templates
	template, ok := a.templates[templateName]
	if !ok {
		log.Printf("Unknown template type %s", templateName)
		return false
	}

	// Add dynamic content to the template
	fillTemplate(template, a.LanguageBundle, lang, content)

	// Email information (subject and body) are retrieved from the "executed" email template
	// "Execution" adds dynamic content using text/template lib
	_, body, err := template.Execute(content)

	if err != nil {
		log.Printf("Error executing email template %s", err)
		return false
	}
	// Get localized subject of email
	subject, err := getLocalizedSubject(a.LanguageBundle, template.Subject(), lang)

	// Finally send the email
	if status, details := a.notifier.Send([]string{conf.Email}, subject, body); status != http.StatusOK {
		log.Printf("Issue sending email: Status [%d] Message [%s]", status, details)
		return false
	}
	return true
}

//find and validate the token
func (a *Api) token(res http.ResponseWriter, req *http.Request) *shoreline.TokenData {
	if token := req.Header.Get(TP_SESSION_TOKEN); token != "" {
		log.Printf("Found token in request header %s", token)
		td := a.sl.CheckToken(token)

		if td == nil {
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusForbidden, STATUS_INVALID_TOKEN)}
			log.Printf("token %s err[%v] ", STATUS_INVALID_TOKEN, statusErr)
			a.sendModelAsResWithStatus(res, statusErr, http.StatusForbidden)
			return nil
		}
		//all good!
		return td
	}
	statusErr := &status.StatusError{Status: status.NewStatus(http.StatusUnauthorized, STATUS_NO_TOKEN)}
	log.Printf("token %s err[%v] ", STATUS_NO_TOKEN, statusErr)
	a.sendModelAsResWithStatus(res, statusErr, http.StatusUnauthorized)
	return nil
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
		log.Printf("Error [%s] trying to get existing users details", err.Error())
		return nil
	} else {
		log.Printf("User found at shoreline using token %s", token)
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
		log.Printf("Error [%s] trying to send model [%s]", err.Error(), model)
		http.Error(res, "Error marshaling data for response", http.StatusInternalServerError)
	} else {
		res.Header().Set("content-type", "application/json")
		res.WriteHeader(statusCode)
		res.Write(jsonDetails)
	}
	return
}

func (a *Api) sendError(res http.ResponseWriter, statusCode int, reason string, extras ...interface{}) {
	_, file, line, ok := runtime.Caller(1)
	if ok {
		segments := strings.Split(file, "/")
		file = segments[len(segments)-1]
	} else {
		file = "???"
		line = 0
	}

	messages := make([]string, len(extras))
	for index, extra := range extras {
		messages[index] = fmt.Sprintf("%v", extra)
	}

	log.Printf("%s:%d RESPONSE ERROR: [%d %s] %s", file, line, statusCode, reason, strings.Join(messages, "; "))
	a.sendModelAsResWithStatus(res, status.NewStatus(statusCode, reason), statusCode)
}

func (a *Api) sendErrorWithCode(res http.ResponseWriter, statusCode int, errorCode int, reason string, extras ...interface{}) {
	_, file, line, ok := runtime.Caller(1)
	if ok {
		segments := strings.Split(file, "/")
		file = segments[len(segments)-1]
	} else {
		file = "???"
		line = 0
	}

	messages := make([]string, len(extras))
	for index, extra := range extras {
		messages[index] = fmt.Sprintf("%v", extra)
	}

	log.Printf("%s:%d RESPONSE ERROR: [%d %s] %s", file, line, statusCode, reason, strings.Join(messages, "; "))
	a.sendModelAsResWithStatus(res, status.NewStatusWithError(statusCode, errorCode, reason), statusCode)
}

func (a *Api) tokenUserHasRequestedPermissions(tokenData *shoreline.TokenData, groupId string, requestedPermissions commonClients.Permissions) (commonClients.Permissions, error) {
	if tokenData.IsServer {
		return requestedPermissions, nil
	} else if tokenData.UserID == groupId {
		return requestedPermissions, nil
	} else if actualPermissions, err := a.gatekeeper.UserInGroup(tokenData.UserID, groupId); err != nil {
		return commonClients.Permissions{}, err
	} else {
		finalPermissions := make(commonClients.Permissions, 0)
		for permission, _ := range requestedPermissions {
			if reflect.DeepEqual(requestedPermissions[permission], actualPermissions[permission]) {
				finalPermissions[permission] = requestedPermissions[permission]
			}
		}
		return finalPermissions, nil
	}
}
