package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/mux"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	crewClient "github.com/mdblp/crew/client"
	"github.com/mdblp/go-common/clients/auth"
	"github.com/mdblp/go-common/clients/portal"
	"github.com/mdblp/go-common/clients/seagull"
	"github.com/mdblp/go-common/clients/status"
	"github.com/mdblp/hydrophone/clients"
	"github.com/mdblp/hydrophone/models"
	"github.com/mdblp/shoreline/clients/shoreline"
	"github.com/mdblp/shoreline/schema"
	"github.com/mdblp/shoreline/token"
)

type (
	Api struct {
		Store          clients.StoreClient
		notifier       clients.Notifier
		templates      models.Templates
		sl             shoreline.ClientInterface
		perms          crewClient.Crew
		auth           auth.ClientInterface
		seagull        seagull.API
		portal         portal.API
		Config         Config
		LanguageBundle *i18n.Bundle
		logger         *log.Logger
	}
	Config struct {
		ServerSecret                   string `json:"serverSecret"`              //used for services
		WebURL                         string `json:"webUrl"`                    // used for link to blip
		SupportURL                     string `json:"supportUrl"`                // used for link to support
		AssetURL                       string `json:"assetUrl"`                  // used for location of the images
		I18nTemplatesPath              string `json:"i18nTemplatesPath"`         // where are the templates located?
		AllowPatientResetPassword      bool   `json:"allowPatientResetPassword"` // true means that patients can reset their password, false means that only clinicianc can reset their password
		PatientPasswordResetURL        string `json:"patientPasswordResetUrl"`   // URL of the help web site that is used to give instructions to reset password for patients
		Protocol                       string `json:"protocol"`
		EnableTestRoutes               bool   `json:"test"`
		ConfirmationAttempts           int64
		ConfirmationAttemptsTimeWindow time.Duration
	}

	group struct {
		Members []string
	}
	// this just makes it easier to bind a handler for the Handle function
	varsHandler func(http.ResponseWriter, *http.Request, map[string]string)
)

const (
	//api logging prefix
	CONFIRM_API_PREFIX = "api/confirm "

	TP_SESSION_TOKEN = "x-tidepool-session-token"
	// TP_TRACE_SESSION Session trace: uuid v4
	TP_TRACE_SESSION = "x-tidepool-trace-session"

	//returned error messages
	STATUS_ERR_SENDING_EMAIL         = "Error sending email"
	STATUS_ERR_SAVING_CONFIRMATION   = "Error saving the confirmation"
	STATUS_ERR_CREATING_CONFIRMATION = "Error creating a confirmation"
	STATUS_ERR_CLINICAL_USR          = "Cannot send an information to clinical"
	STATUS_ERR_TOO_MANY_ATTEMPTS     = "Cannot send confirmation, too many attempts"
	STATUS_ERR_RESET_PWD_FORBIDEN    = "Cannot send reset password to non patients users"
	STATUS_ERR_FINDING_CONFIRMATION  = "Error finding the confirmation"
	STATUS_ERR_FINDING_USER          = "Error finding the user"
	STATUS_ERR_FINDING_TEAM          = "Error finding the team"
	STATUS_ERR_PATIENT_NOT_MBR       = "Error finding the patient in the team"
	STATUS_ERR_DECODING_CONFIRMATION = "Error decoding the confirmation"
	STATUS_ERR_FINDING_PREVIEW       = "Error finding the invite preview"
	STATUS_ERR_FINDING_VALIDATION    = "Error finding the account validation"
	STATUS_ERR_DECODING_INVITE       = "Error decoding the invitation"
	STATUS_ERR_MISSING_DATA_INVITE   = "Error missing data in the invitation"
	STATUS_ERR_INVALID_DATA          = "Error invalid data in the invitation"
	STATUS_ERR_DECODING_BODY         = "Error decoding the message body"
	STATUS_ERR_MONITORED_PATIENT     = "Error on monitored patient"

	//returned status messages
	STATUS_NOT_FOUND           = "Nothing found"
	STATUS_NO_TOKEN            = "No x-tidepool-session-token was found"
	STATUS_INVALID_TOKEN       = "The x-tidepool-session-token was invalid"
	STATUS_UNAUTHORIZED        = "Not authorized for requested operation"
	STATUS_ROLE_ALRDY_ASSIGNED = "Role already assigned to user"
	STATUS_NOT_MEMBER          = "User is not a member"
	STATUS_NOT_ADMIN           = STATUS_UNAUTHORIZED
	STATUS_NOT_TEAM_MONITORING = "Not a monitoring team"
	STATUS_OK                  = "OK"

	STATUS_SIGNUP_NO_ID             = "Required userid is missing"
	STATUS_ERR_FINDING_USR          = "Error finding user"
	STATUS_ERR_UPDATING_USR         = "Error updating user"
	STATUS_ERR_UPDATING_TEAM        = "Error updating team"
	STATUS_ERR_COUNTING_CONF        = "Error counting existing confirmations"
	STATUS_NO_PASSWORD              = "User does not have a password"
	STATUS_MISSING_PASSWORD         = "Password is missing"
	STATUS_INVALID_PASSWORD         = "Password specified is invalid"
	STATUS_MISSING_BIRTHDAY         = "Birthday is missing"
	STATUS_INVALID_BIRTHDAY         = "Birthday specified is invalid"
	STATUS_MISMATCH_BIRTHDAY        = "Birthday specified does not match patient birthday"
	STATUS_PATIENT_NOT_AUTH         = "Patient cannot be member of care team"
	STATUS_MEMBER_NOT_AUTH          = "Non patient users cannot be a patient of care team"
	STATUS_PATIENT_NOT_CAREGIVER    = "Patient cannot be added as caregiver"
	STATUS_ERR_CANCELING_MONITORING = "Error cancelling monitoring"
)

var bmPolicy = bluemonday.StrictPolicy()

func sanitize(el string) string {
	return bmPolicy.Sanitize(el)
}

func InitApi(
	cfg Config,
	store clients.StoreClient,
	ntf clients.Notifier,
	sl shoreline.ClientInterface,
	perms crewClient.Crew,
	auth auth.ClientInterface,
	seagull seagull.API,
	portal portal.API,
	templates models.Templates,
	logger *log.Logger,
) *Api {
	return &Api{
		Store:          store,
		Config:         cfg,
		notifier:       ntf,
		sl:             sl,
		perms:          perms,
		auth:           auth,
		seagull:        seagull,
		portal:         portal,
		templates:      templates,
		LanguageBundle: nil,
		logger:         logger,
	}
}

func (a *Api) getWebURL(req *http.Request) string {
	if a.Config.WebURL == "" {
		host := req.Header.Get("Host")
		return a.Config.Protocol + "://" + host
	}
	return a.Config.WebURL
}

func (a *Api) SetHandlers(prefix string, rtr *mux.Router) {

	rtr.HandleFunc("/status", a.GetStatus).Methods("GET")

	rtr.Handle("/sanity_check/{userid}", varsHandler(a.sendSanityCheckEmail)).Methods("POST")

	// POST /confirm/send/forgot/:useremail
	// POST /confirm/send/invite/:userid
	send := rtr.PathPrefix("/send").Subrouter()
	send.Handle("/forgot/{useremail}", varsHandler(a.passwordReset)).Methods("POST")
	send.Handle("/invite/{userid}", varsHandler(a.SendInvite)).Methods("POST")
	// POST /confirm/send/team/invite
	send.Handle("/team/invite", varsHandler(a.SendTeamInvite)).Methods("POST")
	// POST /confirm/send/team/monitoring/{teamid}/{userid}
	send.Handle("/team/monitoring/{teamid}/{userid}", varsHandler(a.SendMonitoringTeamInvite)).Methods("POST")
	// POST /confirm/send/team/role/:userid - add or remove admin role to userid
	send.Handle("/team/role/{userid}", varsHandler(a.UpdateTeamRole)).Methods("PUT")
	// DELETE /confirm/send/team/leave/:teamid/:userid - delete member
	send.Handle("/team/leave/{teamid}/{userid}", varsHandler(a.DeleteTeamMember)).Methods("DELETE")

	// POST /confirm/send/inform/:userid
	send.Handle("/inform/{userid}", varsHandler(a.sendSignUpInformation)).Methods("POST")
	send.Handle("/pin-reset/{userid}", varsHandler(a.SendPinReset)).Methods("POST")

	// PUT /confirm/accept/forgot/
	// PUT /confirm/accept/invite/:userid/:invited_by
	accept := rtr.PathPrefix("/accept").Subrouter()
	accept.Handle("/forgot", varsHandler(a.acceptPassword)).Methods("PUT")
	accept.Handle("/invite/{userid}/{invitedby}", varsHandler(a.AcceptInvite)).Methods("PUT")
	// PUT /confirm/accept/team/invite
	accept.Handle("/team/invite", varsHandler(a.AcceptTeamNotifs)).Methods("PUT")
	// PUT /confirm/accept/team/monitoring/{teamid}/{userid}
	accept.Handle("/team/monitoring/{teamid}/{userid}", varsHandler(a.AcceptMonitoringInvite)).Methods("PUT")

	// GET /confirm/invite/:userid
	rtr.Handle("/invite/{userid}", varsHandler(a.GetSentInvitations)).Methods("GET")

	// GET /confirm/invitations/:userid
	rtr.Handle("/invitations/{userid}", varsHandler(a.GetReceivedInvitations)).Methods("GET")

	// PUT /confirm/dismiss/invite/:userid/:invited_by
	dismiss := rtr.PathPrefix("/dismiss").Subrouter()
	dismiss.Handle("/invite/{userid}/{invitedby}",
		varsHandler(a.DismissInvite)).Methods("PUT")
	// PUT /confirm/dismiss/team/invite/{teamid}
	dismiss.Handle("/team/invite/{teamid}", varsHandler(a.DismissTeamInvite)).Methods("PUT")
	// PUT /confirm/dismiss/team/monitoring/{teamid}/{userid}
	dismiss.Handle("/team/monitoring/{teamid}/{userid}", varsHandler(a.DismissMonitoringInvite)).Methods("PUT")

	rtr.Handle("/cancel/invite", varsHandler(a.CancelAnyInvite)).Methods("POST")
	if a.Config.EnableTestRoutes {
		rtr.Handle("/cancel/all/{email}", varsHandler(a.CancelAllInvites)).Methods("POST")
	}

	// PUT /confirm/:userid/invited/:invited_address
	rtr.Handle("/{userid}/invited/{invited_address}", varsHandler(a.CancelInvite)).Methods("PUT")
}

func (h varsHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	h(res, req, vars)
}

func getSessionToken(req *http.Request) string {
	// time:= c.Params.ByName("time")
	sessionToken := req.Header.Get(token.TP_SESSION_TOKEN)
	if sessionToken != "" {
		return sessionToken
	}
	sessionToken = strings.Trim(req.Header.Get("Authorization"), " ")
	if sessionToken != "" && strings.HasPrefix(sessionToken, "Bearer ") {
		tokenParts := strings.Split(sessionToken, " ")
		sessionToken = tokenParts[1]
	}
	return sessionToken
}

// @Summary Get the api status
// @Description Get the api status
// @ID hydrophone-api-getstatus
// @Accept  json
// @Produce  json
// @Success 200 {string} string "OK"
// @Failure 500 {string} string "error description"
// @Router /status [get]
func (a *Api) GetStatus(res http.ResponseWriter, req *http.Request) {
	var s status.ApiStatus
	if err := a.Store.Ping(); err != nil {
		log.Printf("Error getting status [%v]", err)
		s = status.NewApiStatus(http.StatusInternalServerError, err.Error())
	} else {
		s = status.NewApiStatus(http.StatusOK, "OK")
	}
	a.sendModelAsResWithStatus(res, s, s.Status.Code)
	return
}

//Save this confirmation or
//write an error if it all goes wrong
func (a *Api) addOrUpdateConfirmation(ctx context.Context, conf *models.Confirmation, res http.ResponseWriter) bool {
	if err := a.Store.UpsertConfirmation(ctx, conf); err != nil {
		log.Printf("Error saving the confirmation [%v]", err)
		statusErr := &status.StatusError{status.NewStatus(http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusInternalServerError)
		return false
	}
	return true
}

//Find this confirmation
//write error if it fails
func (a *Api) findExistingConfirmation(ctx context.Context, conf *models.Confirmation, res http.ResponseWriter) (*models.Confirmation, error) {
	if found, err := a.Store.FindConfirmation(ctx, conf); err != nil {
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
		//log.Println("No confirmations were found ", statusErr.Error())
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
func (a *Api) createAndSendNotification(req *http.Request, conf *models.Confirmation, content map[string]string, lang string) bool {
	log.Printf("trying notification with template '%s' to %s with language '%s'", conf.TemplateName, conf.Email, lang)

	// Get the template name based on the requested communication type
	templateName := conf.TemplateName
	if templateName == models.TemplateNameUndefined {
		switch conf.Type {
		case models.TypePasswordReset:
			templateName = models.TemplateNamePasswordReset
		case models.TypePatientPasswordReset:
			templateName = models.TemplateNamePatientPasswordReset
		case models.TypePatientPasswordInfo:
			templateName = models.TemplateNamePatientPasswordInfo
		case models.TypeCareteamInvite:
			templateName = models.TemplateNameCareteamInvite
		case models.TypeMedicalTeamInvite:
			templateName = models.TemplateNameMedicalteamInvite
		case models.TypeMedicalTeamPatientInvite:
			templateName = models.TemplateNameMedicalteamPatientInvite
		case models.TypeMedicalTeamMonitoringInvite:
			templateName = models.TemplateNameMedicalteamMonitoringInvite
		case models.TypeMedicalTeamDoAdmin:
			templateName = models.TemplateNameMedicalteamDoAdmin
		case models.TypeMedicalTeamRemove:
			templateName = models.TemplateNameMedicalteamRemove
		case models.TypeSignUp:
			templateName = models.TemplateNameSignup
		case models.TypeNoAccount:
			templateName = models.TemplateNameNoAccount
		case models.TypeInformation:
			templateName = models.TemplateNamePatientInformation
		default:
			log.Printf("Unknown confirmation type %s", conf.Type)
			return false
		}
	}

	// Content collection is here to replace placeholders in template body/content
	content["WebURL"] = a.getWebURL(req)
	content["SupportURL"] = a.Config.SupportURL
	content["AssetURL"] = a.Config.AssetURL
	content["PatientPasswordResetURL"] = a.Config.PatientPasswordResetURL
	content["SupportEmail"] = a.Config.SupportURL

	mail, ok := content["Email"]
	if ok {
		content["EncodedEmail"] = url.QueryEscape(mail)
	}

	// Retrieve the template from all the preloaded templates
	template, ok := a.templates[templateName]
	if !ok {
		log.Printf("Unknown template type %s", templateName)
		return false
	}

	// Email information (subject and body) are retrieved from the "executed" email template
	// "Execution" adds dynamic content using text/template lib
	subject, body, err := template.Execute(content, lang)

	if err != nil {
		log.Printf("Error executing email template '%s'", err)
		return false
	}

	var tags = make(map[string]string)
	tags["hydrophoneTemplate"] = templateName.String()
	if traceSession := req.Header.Get(TP_TRACE_SESSION); traceSession != "" {
		tags[TP_TRACE_SESSION] = traceSession
	}

	// Finally send the email
	if status, details := a.notifier.Send([]string{conf.Email}, subject, body, tags); status != http.StatusOK {
		log.Printf("Issue sending email: Status [%d] Message [%s]", status, details)
		return false
	}
	return true
}

//find and validate the token
func (a *Api) token(res http.ResponseWriter, req *http.Request) *token.TokenData {
	td := a.auth.Authenticate(req)

	if td == nil {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusUnauthorized, STATUS_NO_TOKEN)}
		a.sendModelAsResWithStatus(res, statusErr, http.StatusUnauthorized)
		return nil
	}
	//all good!
	return td
}

// logAudit Variatic log for audit trails
func (a *Api) logAudit(req *http.Request, format string, args ...interface{}) {
	var prefix string

	// Get token from request
	td := a.auth.Authenticate(req)
	isServer := td != nil && td.IsServer

	if req.RemoteAddr != "" {
		prefix = fmt.Sprintf("remoteAddr{%s}, ", req.RemoteAddr)
	}

	traceSession := req.Header.Get(TP_TRACE_SESSION)
	if traceSession != "" {
		prefix += fmt.Sprintf("trace{%s}, ", traceSession)
	}

	prefix += fmt.Sprintf("isServer{%t}, ", isServer)

	s := fmt.Sprintf(format, args...)
	a.logger.Printf("%s%s", prefix, s)
}

//Find existing user based on the given identifier
//The identifier could be either an id or email address
func (a *Api) findExistingUser(identifier, token string) *schema.UserData {
	if usr, err := a.sl.GetUser(identifier, token); err != nil {
		log.Printf("Error [%s] trying to get existing users details", err.Error())
		return nil
	} else {
		log.Printf("User found at shoreline using token %s", token)
		return usr
	}
}

//Makesure we have set the userId on these confirmations
func (a *Api) ensureIdSet(ctx context.Context, userId string, confirmations []*models.Confirmation) {

	if len(confirmations) < 1 {
		return
	}
	for i := range confirmations {
		//set the userid if not set already
		if confirmations[i].UserId == "" {
			log.Println("UserId wasn't set for invite so setting it")
			confirmations[i].UserId = userId
			a.Store.UpsertConfirmation(ctx, confirmations[i])
		}
	}
	return
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

func (a *Api) isAuthorizedUser(tokenData *token.TokenData, userId string) bool {
	if tokenData.IsServer {
		return true
	} else if tokenData.UserId == userId {
		return true
	} else {
		return false
	}
}

func (a *Api) verifySendAttempts(ctx context.Context, confirmationType models.Type, creatorId string, email string, userId string) (bool, int64, error) {
	confirm := models.Confirmation{
		Type:      confirmationType,
		CreatorId: creatorId,
		Email:     email,
		UserId:    userId,
	}
	createdSince := time.Now().Add(-a.Config.ConfirmationAttemptsTimeWindow)
	var count int64
	var err error
	if confirm.Type == models.TypeSignUp {
		var res *models.Confirmation
		res, err = a.Store.FindConfirmation(ctx, &confirm)
		if err == nil {
			if res == nil {
				count = 0
			} else if res.Created.After(createdSince) {
				count = res.ResendCounter
			}
		}
	} else {
		count, err = a.Store.CountLatestConfirmations(ctx, confirm, createdSince)
		a.logger.Printf("verifySendAttempts::count:%v for %v", count, confirm)
	}
	if err != nil {
		return false, 0, err
	}
	if count >= a.Config.ConfirmationAttempts {
		return false, count, nil
	}
	return true, count, nil
}
