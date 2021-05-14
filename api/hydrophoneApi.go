package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"

	"github.com/gorilla/mux"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	crewClient "github.com/mdblp/crew/client"
	"github.com/mdblp/crew/store"
	"github.com/mdblp/shoreline/clients/shoreline"
	"github.com/mdblp/shoreline/schema"
	"github.com/mdblp/shoreline/token"
	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/portal"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/hydrophone/clients"
	"github.com/tidepool-org/hydrophone/models"
)

type (
	Api struct {
		Store          clients.StoreClient
		notifier       clients.Notifier
		templates      models.Templates
		sl             shoreline.ClientInterface
		perms          crewClient.Crew
		seagull        commonClients.Seagull
		portal         portal.Client
		Config         Config
		LanguageBundle *i18n.Bundle
		logger         *log.Logger
	}
	Config struct {
		ServerSecret              string `json:"serverSecret"`              //used for services
		WebURL                    string `json:"webUrl"`                    // used for link to blip
		SupportURL                string `json:"supportUrl"`                // used for link to support
		AssetURL                  string `json:"assetUrl"`                  // used for location of the images
		I18nTemplatesPath         string `json:"i18nTemplatesPath"`         // where are the templates located?
		AllowPatientResetPassword bool   `json:"allowPatientResetPassword"` // true means that patients can reset their password, false means that only clinicianc can reset their password
		PatientPasswordResetURL   string `json:"patientPasswordResetUrl"`   // URL of the help web site that is used to give instructions to reset password for patients
		Protocol                  string `json:"protocol"`
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
	STATUS_ERR_FINDING_CONFIRMATION  = "Error finding the confirmation"
	STATUS_ERR_FINDING_USER          = "Error finding the user"
	STATUS_ERR_FINDING_TEAM          = "Error finding the team"
	STATUS_ERR_DECODING_CONFIRMATION = "Error decoding the confirmation"
	STATUS_ERR_FINDING_PREVIEW       = "Error finding the invite preview"
	STATUS_ERR_FINDING_VALIDATION    = "Error finding the account validation"
	STATUS_ERR_DECODING_INVITE       = "Error decoding the invitation"
	STATUS_ERR_MISSING_DATA_INVITE   = "Error missing data in the invitation"

	//returned status messages
	STATUS_NOT_FOUND           = "Nothing found"
	STATUS_NO_TOKEN            = "No x-tidepool-session-token was found"
	STATUS_INVALID_TOKEN       = "The x-tidepool-session-token was invalid"
	STATUS_UNAUTHORIZED        = "Not authorized for requested operation"
	STATUS_ROLE_ALRDY_ASSIGNED = "Role already assigned to user"
	STATUS_NOT_MEMBER          = "User is not a member"
	STATUS_NOT_ADMIN           = STATUS_UNAUTHORIZED
	STATUS_OK                  = "OK"
)

func InitApi(
	cfg Config,
	store clients.StoreClient,
	ntf clients.Notifier,
	sl shoreline.ClientInterface,
	perms crewClient.Crew,
	seagull commonClients.Seagull,
	portal portal.Client,
	templates models.Templates,
) *Api {
	logger := log.New(os.Stdout, CONFIRM_API_PREFIX, log.LstdFlags)
	return &Api{
		Store:          store,
		Config:         cfg,
		notifier:       ntf,
		sl:             sl,
		perms:          perms,
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

	// POST /confirm/send/signup/:userid
	// POST /confirm/send/forgot/:useremail
	// POST /confirm/send/invite/:userid
	send := rtr.PathPrefix("/send").Subrouter()
	send.Handle("/signup/{userid}", varsHandler(a.sendSignUp)).Methods("POST")
	send.Handle("/forgot/{useremail}", varsHandler(a.passwordReset)).Methods("POST")
	send.Handle("/invite/{userid}", varsHandler(a.SendInvite)).Methods("POST")
	// POST /confirm/send/team/invite
	send.Handle("/team/invite", varsHandler(a.SendTeamInvite)).Methods("POST")
	// POST /confirm/send/team/role/:userid - add or remove admin role to userid
	send.Handle("/team/role/{userid}", varsHandler(a.UpdateTeamRole)).Methods("PUT")
	// DELETE /confirm/send/team/leave/:userid - delete member
	send.Handle("/team/leave/{userid}", varsHandler(a.DeleteTeamMember)).Methods("DELETE")

	// POST /confirm/send/inform/:userid
	send.Handle("/inform/{userid}", varsHandler(a.sendSignUpInformation)).Methods("POST")
	send.Handle("/pin-reset/{userid}", varsHandler(a.SendPinReset)).Methods("POST")

	// POST /confirm/resend/signup/:useremail
	rtr.Handle("/resend/signup/{useremail}", varsHandler(a.resendSignUp)).Methods("POST")

	// PUT /confirm/accept/signup/:confirmationID
	// PUT /confirm/accept/forgot/
	// PUT /confirm/accept/invite/:userid/:invited_by
	accept := rtr.PathPrefix("/accept").Subrouter()
	accept.Handle("/signup/{confirmationid}", varsHandler(a.acceptSignUp)).Methods("PUT")
	accept.Handle("/forgot", varsHandler(a.acceptPassword)).Methods("PUT")
	accept.Handle("/invite/{userid}/{invitedby}", varsHandler(a.AcceptInvite)).Methods("PUT")
	// PUT /confirm/accept/team/invite
	accept.Handle("/team/invite", varsHandler(a.AcceptTeamNotifs)).Methods("PUT")

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
	// PUT /confirm/dismiss/team/invite/{teamid}
	dismiss.Handle("/team/invite/{teamid}", varsHandler(a.DismissTeamInvite)).Methods("PUT")

	// PUT /confirm/:userid/invited/:invited_address
	// PUT /confirm/signup/:userid
	rtr.Handle("/{userid}/invited/{invited_address}", varsHandler(a.CancelInvite)).Methods("PUT")
	rtr.Handle("/signup/{userid}", varsHandler(a.cancelSignUp)).Methods("PUT")
}

func (h varsHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	h(res, req, vars)
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
func (a *Api) checkFoundConfirmations(token string, res http.ResponseWriter, results []*models.Confirmation, err error) []*models.Confirmation {
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
			if results[i].Team != nil && results[i].Team.ID != "" {
				team, err := a.perms.GetTeam(token, results[i].Team.ID)
				if err != nil {
					log.Println("Error getting team", err.Error())
				} else {
					results[i].Team.Name = team.Name
				}
			}
		}
		return results
	}
}

//Generate a notification from the given confirmation,write the error if it fails
func (a *Api) createAndSendNotification(req *http.Request, conf *models.Confirmation, content map[string]interface{}, lang string) bool {
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

	// Support address configuration contains the mailto we want to strip out
	supportEmail := fmt.Sprintf("<a href=%s>%s</a>", a.Config.SupportURL, strings.Replace(a.Config.SupportURL, "mailto:", "", 1))

	// Content collection is here to replace placeholders in template body/content
	content["WebURL"] = a.getWebURL(req)
	content["SupportURL"] = a.Config.SupportURL
	content["AssetURL"] = a.Config.AssetURL
	content["PatientPasswordResetURL"] = a.Config.PatientPasswordResetURL
	content["SupportEmail"] = supportEmail

	mail, ok := content["Email"]
	if ok {
		content["EncodedEmail"] = url.QueryEscape(mail.(string))
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

	// Finally send the email
	if status, details := a.notifier.Send([]string{conf.Email}, subject, body); status != http.StatusOK {
		log.Printf("Issue sending email: Status [%d] Message [%s]", status, details)
		return false
	}
	return true
}

//find and validate the token
func (a *Api) token(res http.ResponseWriter, req *http.Request) *token.TokenData {
	if token := req.Header.Get(TP_SESSION_TOKEN); token != "" {
		log.Printf("Found token in request header %s", token)
		td := a.sl.CheckToken(token)

		if td == nil {
			statusErr := &status.StatusError{Status: status.NewStatus(http.StatusForbidden, STATUS_INVALID_TOKEN)}
			log.Printf("token %v err[%v] ", token, statusErr)
			a.sendModelAsResWithStatus(res, statusErr, http.StatusForbidden)
			return nil
		}
		//all good!
		return td
	}
	statusErr := &status.StatusError{Status: status.NewStatus(http.StatusUnauthorized, STATUS_NO_TOKEN)}
	a.sendModelAsResWithStatus(res, statusErr, http.StatusUnauthorized)
	return nil
}

// logAudit Variatic log for audit trails
func (a *Api) logAudit(req *http.Request, format string, args ...interface{}) {
	var prefix string
	var isServer bool = false

	// Get token from request
	if token := req.Header.Get(TP_SESSION_TOKEN); token != "" {
		td := a.sl.CheckToken(token)
		isServer = td != nil && td.IsServer
	}

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

// userId is member of a Team
// Settings the all parameter to true will return all the members while it will only return the accepted members if the parameter is set false
func (a *Api) isTeamMember(userID string, team store.Team, all bool) bool {
	for i := 0; i < len(team.Members); i++ {
		if team.Members[i].UserID == userID && (team.Members[i].InvitationStatus == "accepted" || all) {
			return true
		}
	}
	return false
}

//
// return true is the user userID is admin of the Team identified by teamID
// it returns the Team object corresponding to the team
// if any error occurs during the search, it returns an error with the
// related code
func (a *Api) getTeamForUser(token, teamID, userID string, res http.ResponseWriter) (bool, store.Team, error) {
	var auth = false
	team, err := a.perms.GetTeam(token, teamID)
	if err != nil {
		statusErr := &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_ERR_FINDING_TEAM)}
		a.sendModelAsResWithStatus(res, statusErr, statusErr.Code)
		return auth, store.Team{}, err
	}
	auth = a.isTeamAdmin(userID, *team)
	return auth, *team, nil
}
