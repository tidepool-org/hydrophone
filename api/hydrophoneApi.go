package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	clinicsClient "github.com/tidepool-org/clinic/client"
	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/highwater"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/hydrophone/clients"
	"github.com/tidepool-org/hydrophone/models"
	"github.com/tidepool-org/platform/alerts"
)

type (
	Api struct {
		Store      clients.StoreClient
		clinics    clinicsClient.ClientWithResponsesInterface
		notifier   clients.Notifier
		templates  models.Templates
		sl         shoreline.Client
		gatekeeper commonClients.Gatekeeper
		seagull    commonClients.Seagull
		metrics    highwater.Client
		alerts     AlertsClient
		baseLogger *zap.SugaredLogger
		Config     Config
		mu         sync.Mutex
	}
	Config struct {
		ServerSecret string `envconfig:"TIDEPOOL_SERVER_SECRET" required:"true"`
		WebUrl       string `split_words:"true" required:"true"`
		AssetUrl     string `split_words:"true" required:"true"`
		Protocol     string `default:"http"`
	}

	// this just makes it easier to bind a handler for the Handle function
	varsHandler func(http.ResponseWriter, *http.Request, map[string]string)
)

type AlertsClient interface {
	Upsert(context.Context, *alerts.Config) error
	Delete(context.Context, *alerts.Config) error
}

const (
	TP_SESSION_TOKEN = "x-tidepool-session-token"

	STATUS_ERR_ACCEPTING_CONFIRMATION = "Error accepting invitation"
	STATUS_ERR_ADDING_PROFILE         = "Error adding profile"
	STATUS_ERR_CREATING_ALERTS_CONFIG = "Error creating alerts configuration"
	STATUS_ERR_CREATING_CONFIRMATION  = "Error creating a confirmation"
	STATUS_ERR_CREATING_PATIENT       = "Error creating patient"
	STATUS_ERR_DECODING_CONFIRMATION  = "Error decoding the confirmation"
	STATUS_ERR_DECODING_CONTEXT       = "Error decoding the confirmation context"
	STATUS_ERR_DELETING_CONFIRMATION  = "Error deleting a confirmation"
	STATUS_ERR_FINDING_CLINIC         = "Error finding the clinic"
	STATUS_ERR_FINDING_CONFIRMATION   = "Error finding the confirmation"
	STATUS_ERR_FINDING_PREVIEW        = "Error finding the invite preview"
	STATUS_ERR_FINDING_USER           = "Error finding the user"
	STATUS_ERR_RESETTING_KEY          = "Error resetting key"
	STATUS_ERR_SAVING_CONFIRMATION    = "Error saving the confirmation"
	STATUS_ERR_SENDING_EMAIL          = "Error sending email"
	STATUS_ERR_SETTING_PERMISSIONS    = "Error setting permissions"
	STATUS_ERR_UPDATING_USER          = "Error updating user"
	STATUS_ERR_VALIDATING_CONTEXT     = "Error validating the confirmation context"

	STATUS_EXISTING_SIGNUP   = "User already has an existing valid signup confirmation"
	STATUS_INVALID_BIRTHDAY  = "Birthday specified is invalid"
	STATUS_INVALID_PASSWORD  = "Password specified is invalid"
	STATUS_INVALID_TOKEN     = "The x-tidepool-session-token was invalid"
	STATUS_MISMATCH_BIRTHDAY = "Birthday specified does not match patient birthday"
	STATUS_MISSING_BIRTHDAY  = "Birthday is missing"
	STATUS_MISSING_PASSWORD  = "Password is missing"
	STATUS_NO_PASSWORD       = "User does not have a password"
	STATUS_NO_TOKEN          = "No x-tidepool-session-token was found"
	STATUS_OK                = "OK"
	STATUS_SIGNUP_ACCEPTED   = "User has had signup confirmed"
	STATUS_SIGNUP_ERROR      = "Error while completing signup confirmation. The signup confirmation remains active until it expires"
	STATUS_SIGNUP_EXPIRED    = "The signup confirmation has expired"
	STATUS_SIGNUP_NOT_FOUND  = "No matching signup confirmation was found"
	STATUS_SIGNUP_NO_CONF    = "Required confirmation id is missing"
	STATUS_SIGNUP_NO_ID      = "Required userid is missing"
	STATUS_UNAUTHORIZED      = "Not authorized for requested operation"
	STATUS_NOT_FOUND         = "Nothing found"

	ERROR_NO_PASSWORD       = 1001
	ERROR_MISSING_PASSWORD  = 1002
	ERROR_INVALID_PASSWORD  = 1003
	ERROR_MISSING_BIRTHDAY  = 1004
	ERROR_INVALID_BIRTHDAY  = 1005
	ERROR_MISMATCH_BIRTHDAY = 1006
)

func NewApi(
	cfg Config,
	clinics clinicsClient.ClientWithResponsesInterface,
	store clients.StoreClient,
	ntf clients.Notifier,
	sl shoreline.Client,
	gatekeeper commonClients.Gatekeeper,
	metrics highwater.Client,
	seagull commonClients.Seagull,
	alerts AlertsClient,
	templates models.Templates,
	logger *zap.SugaredLogger,
) *Api {
	return &Api{
		Store:      store,
		Config:     cfg,
		clinics:    clinics,
		notifier:   ntf,
		sl:         sl,
		gatekeeper: gatekeeper,
		metrics:    metrics,
		seagull:    seagull,
		alerts:     alerts,
		templates:  templates,
		baseLogger: logger,
	}
}

func (a *Api) getWebURL(req *http.Request) string {
	if a.Config.WebUrl == "" {
		host := req.Header.Get("Host")
		return a.Config.Protocol + "://" + host
	}
	return a.Config.WebUrl
}

func apiConfigProvider() (Config, error) {
	var config Config
	err := envconfig.Process("hydrophone", &config)
	if err != nil {
		return Config{}, err
	}
	return config, nil
}

func routerProvider(api *Api) *mux.Router {
	rtr := mux.NewRouter()
	api.SetHandlers("", rtr)
	return rtr
}

// RouterModule build a router
var RouterModule = fx.Options(fx.Provide(routerProvider, apiConfigProvider))

// addPathVarToLogger adds a request's path variable to the logging context.
//
// It uses the first case-insensitive match of name it finds, additional occurrences of name are
// ignored.
func (a *Api) addPathVarToLogger(name string) mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, orig *http.Request) {
			vars := mux.Vars(orig)
			next := orig
			for key := range vars {
				if !strings.EqualFold(key, name) {
					continue
				}
				ctxLog := a.logger(orig.Context()).With(zap.String(key, vars[key]))
				ctxWithLog := context.WithValue(orig.Context(), ctxLoggerKey{}, ctxLog)
				next = orig.WithContext(ctxWithLog)
				break
			}
			h.ServeHTTP(w, next)
		})
	}
}

type ctxLoggerKey struct{}

func (a *Api) logger(ctx context.Context) *zap.SugaredLogger {
	if logger, ok := ctx.Value(ctxLoggerKey{}).(*zap.SugaredLogger); ok {
		return logger
	}
	return a.cloneLogger()
}

func (a *Api) cloneLogger() *zap.SugaredLogger {
	return a.baseLogger.WithOptions()
}

func (a *Api) ctxLoggerHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origCtx := r.Context()
		ctxLog := a.cloneLogger()
		ctxWithLog := context.WithValue(origCtx, ctxLoggerKey{}, ctxLog)
		rWithLog := r.WithContext(ctxWithLog)
		h.ServeHTTP(w, rWithLog)
	})
}

func (a *Api) SetHandlers(prefix string, rtr *mux.Router) {
	rtr.Use(mux.MiddlewareFunc(a.ctxLoggerHandler))
	rtr.Use(a.addPathVarToLogger("userid"))
	rtr.Use(a.addPathVarToLogger("clinicid"))

	c := rtr.PathPrefix("/confirm").Subrouter()

	c.HandleFunc("/status", a.IsReady).Methods("GET")
	rtr.HandleFunc("/status", a.IsReady).Methods("GET")

	c.HandleFunc("/ready", a.IsReady).Methods("GET")
	rtr.HandleFunc("/ready", a.IsReady).Methods("GET")

	c.HandleFunc("/live", a.IsAlive).Methods("GET")
	rtr.HandleFunc("/live", a.IsAlive).Methods("GET")

	// vars is a shorthand for applying the varsHandler to an handler.
	type vars = varsHandler

	// POST /confirm/send/signup/:userid
	// POST /confirm/send/forgot/:useremail
	// POST /confirm/send/invite/:userid
	csend := rtr.PathPrefix("/confirm/send").Subrouter()
	csend.Handle("/signup/{userid}", vars(a.sendSignUp)).Methods("POST")
	csend.Handle("/forgot/{useremail}", vars(a.passwordReset)).Methods("POST")
	csend.Handle("/invite/{userid}", vars(a.SendInvite)).Methods("POST")
	csend.Handle("/invite/{userId}/clinic", vars(a.InviteClinic)).Methods("POST")

	send := rtr.PathPrefix("/send").Subrouter()
	send.Handle("/signup/{userid}", vars(a.sendSignUp)).Methods("POST")
	send.Handle("/forgot/{useremail}", vars(a.passwordReset)).Methods("POST")
	send.Handle("/invite/{userid}", vars(a.SendInvite)).Methods("POST")
	send.Handle("/invite/{userId}/clinic", vars(a.InviteClinic)).Methods("POST")

	// POST /confirm/resend/signup/:useremail
	// POST /confirm/resend/invite/:inviteId
	c.Handle("/resend/signup/{useremail}", vars(a.resendSignUp)).Methods("POST")
	c.Handle("/resend/invite/{inviteId}", vars(a.ResendInvite)).Methods("PATCH")

	rtr.Handle("/resend/signup/{useremail}", vars(a.resendSignUp)).Methods("POST")
	rtr.Handle("/resend/invite/{inviteId}", vars(a.ResendInvite)).Methods("PATCH")

	// PUT /confirm/accept/signup/:confirmationID
	// PUT /confirm/accept/forgot/
	// PUT /confirm/accept/invite/:userid/:invited_by
	caccept := rtr.PathPrefix("/confirm/accept").Subrouter()
	caccept.Handle("/signup/{confirmationid}", vars(a.acceptSignUp)).Methods("PUT")
	caccept.Handle("/forgot", vars(a.acceptPassword)).Methods("PUT")
	caccept.Handle("/invite/{userid}/{invitedby}", vars(a.AcceptInvite)).Methods("PUT")

	accept := rtr.PathPrefix("/accept").Subrouter()
	accept.Handle("/signup/{confirmationid}", vars(a.acceptSignUp)).Methods("PUT")
	accept.Handle("/forgot", vars(a.acceptPassword)).Methods("PUT")
	accept.Handle("/invite/{userid}/{invitedby}", vars(a.AcceptInvite)).Methods("PUT")

	// GET /confirm/signup/:userid
	// GET /confirm/invite/:userid
	c.Handle("/signup/{userid}", vars(a.getSignUp)).Methods("GET")
	c.Handle("/invite/{userid}", vars(a.GetSentInvitations)).Methods("GET")

	rtr.Handle("/signup/{userid}", vars(a.getSignUp)).Methods("GET")
	rtr.Handle("/invite/{userid}", vars(a.GetSentInvitations)).Methods("GET")

	// GET /confirm/invitations/:userid
	c.Handle("/invitations/{userid}", vars(a.GetReceivedInvitations)).Methods("GET")

	rtr.Handle("/invitations/{userid}", vars(a.GetReceivedInvitations)).Methods("GET")

	// PUT /confirm/dismiss/invite/:userid/:invited_by
	// PUT /confirm/dismiss/signup/:userid
	cdismiss := rtr.PathPrefix("/confirm/dismiss").Subrouter()
	cdismiss.Handle("/invite/{userid}/{invitedby}", vars(a.DismissInvite)).Methods("PUT")
	cdismiss.Handle("/signup/{userid}", vars(a.dismissSignUp)).Methods("PUT")

	dismiss := rtr.PathPrefix("/dismiss").Subrouter()
	dismiss.Handle("/invite/{userid}/{invitedby}", vars(a.DismissInvite)).Methods("PUT")
	dismiss.Handle("/signup/{userid}", vars(a.dismissSignUp)).Methods("PUT")

	// POST /confirm/signup/:userid
	c.Handle("/signup/{userid}", vars(a.createSignUp)).Methods("POST")

	// PUT /confirm/:userid/invited/:invited_address
	// PUT /confirm/signup/:userid
	c.Handle("/{userid}/invited/{invited_address}", vars(a.CancelInvite)).Methods("PUT")
	c.Handle("/signup/{userid}", vars(a.cancelSignUp)).Methods("PUT")

	rtr.Handle("/{userid}/invited/{invited_address}", vars(a.CancelInvite)).Methods("PUT")
	rtr.Handle("/signup/{userid}", vars(a.cancelSignUp)).Methods("PUT")

	// GET /v1/clinics/:clinicId/invites/patients
	// GET /v1/clinics/:clinicId/invites/patients/:inviteId
	c.Handle("/v1/clinics/{clinicId}/invites/patients", vars(a.GetPatientInvites)).Methods("GET")
	c.Handle("/v1/clinics/{clinicId}/invites/patients/{inviteId}", vars(a.AcceptPatientInvite)).Methods("PUT")
	c.Handle("/v1/clinics/{clinicId}/invites/patients/{inviteId}", vars(a.CancelOrDismissPatientInvite)).Methods("DELETE")

	rtr.Handle("/v1/clinics/{clinicId}/invites/patients", vars(a.GetPatientInvites)).Methods("GET")
	rtr.Handle("/v1/clinics/{clinicId}/invites/patients/{inviteId}", vars(a.AcceptPatientInvite)).Methods("PUT")
	rtr.Handle("/v1/clinics/{clinicId}/invites/patients/{inviteId}", vars(a.CancelOrDismissPatientInvite)).Methods("DELETE")

	c.Handle("/v1/clinicians/{userId}/invites", vars(a.GetClinicianInvitations)).Methods("GET")
	c.Handle("/v1/clinicians/{userId}/invites/{inviteId}", vars(a.AcceptClinicianInvite)).Methods("PUT")
	c.Handle("/v1/clinicians/{userId}/invites/{inviteId}", vars(a.DismissClinicianInvite)).Methods("DELETE")

	rtr.Handle("/v1/clinicians/{userId}/invites", vars(a.GetClinicianInvitations)).Methods("GET")
	rtr.Handle("/v1/clinicians/{userId}/invites/{inviteId}", vars(a.AcceptClinicianInvite)).Methods("PUT")
	rtr.Handle("/v1/clinicians/{userId}/invites/{inviteId}", vars(a.DismissClinicianInvite)).Methods("DELETE")

	c.Handle("/v1/clinics/{clinicId}/invites/clinicians", vars(a.SendClinicianInvite)).Methods("POST")
	c.Handle("/v1/clinics/{clinicId}/invites/clinicians/{inviteId}", vars(a.ResendClinicianInvite)).Methods("PATCH")
	c.Handle("/v1/clinics/{clinicId}/invites/clinicians/{inviteId}", vars(a.GetClinicianInvite)).Methods("GET")
	c.Handle("/v1/clinics/{clinicId}/invites/clinicians/{inviteId}", vars(a.CancelClinicianInvite)).Methods("DELETE")

	rtr.Handle("/v1/clinics/{clinicId}/invites/clinicians", vars(a.SendClinicianInvite)).Methods("POST")
	rtr.Handle("/v1/clinics/{clinicId}/invites/clinicians/{inviteId}", vars(a.GetClinicianInvite)).Methods("GET")
	rtr.Handle("/v1/clinics/{clinicId}/invites/clinicians/{inviteId}", vars(a.ResendClinicianInvite)).Methods("PATCH")
	rtr.Handle("/v1/clinics/{clinicId}/invites/clinicians/{inviteId}", vars(a.CancelClinicianInvite)).Methods("DELETE")
}

func (h varsHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	h(res, req, vars)
}

func (a *Api) IsReady(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if err := a.Store.Ping(ctx); err != nil {
		a.sendError(ctx, res, http.StatusInternalServerError, "store connectivity failure", err)
		return
	}
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(STATUS_OK))
}

func (a *Api) IsAlive(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(STATUS_OK))
}

// Save this confirmation or
// write an error if it all goes wrong
func (a *Api) addOrUpdateConfirmation(ctx context.Context, conf *models.Confirmation, res http.ResponseWriter) bool {
	if err := a.Store.UpsertConfirmation(ctx, conf); err != nil {
		a.sendError(ctx, res, http.StatusInternalServerError, STATUS_ERR_SAVING_CONFIRMATION, err)
		return false
	}
	return true
}

// Find this confirmation
// write error if it fails
func (a *Api) addProfile(conf *models.Confirmation) error {
	if conf.CreatorId != "" {
		if err := a.seagull.GetCollection(conf.CreatorId, "profile", a.sl.TokenProvide(), &conf.Creator.Profile); err != nil {
			return err
		}

		conf.Creator.UserId = conf.CreatorId
	}
	return nil
}

func (a *Api) addProfileInfoToConfirmations(ctx context.Context, results []*models.Confirmation) []*models.Confirmation {
	for i := range results {
		if err := a.addProfile(results[i]); err != nil {
			//report and move on
			a.logger(ctx).With(zap.Error(err)).Warn("getting profile")
		}
	}
	return results
}

// Generate a notification from the given confirmation,write the error if it fails
func (a *Api) createAndSendNotification(req *http.Request, conf *models.Confirmation, content map[string]interface{}, recipients ...string) bool {
	ctx := req.Context()
	templateName := conf.TemplateName
	if templateName == models.TemplateNameUndefined {
		switch conf.Type {
		case models.TypePasswordReset:
			templateName = models.TemplateNamePasswordReset
		case models.TypeCareteamInvite:
			templateName = models.TemplateNameCareteamInvite
			has, err := conf.HasPermission("follow")
			if err != nil {
				a.logger(ctx).With(zap.Error(err)).Warn("permissions check failed; falling back to non-alerting notification")
			} else if has {
				templateName = models.TemplateNameCareteamInviteWithAlerting
			}
		case models.TypeSignUp:
			templateName = models.TemplateNameSignup
		case models.TypeNoAccount:
			templateName = models.TemplateNameNoAccount
		default:
			a.logger(ctx).With(zap.String("type", string(conf.Type))).
				Info("unknown confirmation type")
			return false
		}
	}

	content["WebURL"] = a.getWebURL(req)
	content["AssetURL"] = a.Config.AssetUrl

	template, ok := a.templates[templateName]
	if !ok {
		a.logger(ctx).With(zap.String("template", string(templateName))).
			Info("unknown template type")
		return false
	}

	subject, body, err := template.Execute(content)
	if err != nil {
		a.logger(ctx).With(zap.Error(err)).Error("executing email template")
		return false
	}

	addresses := recipients
	if conf.Email != "" {
		addresses = append(recipients, conf.Email)
	}
	if len(addresses) == 0 {
		return true
	}

	if status, details := a.notifier.Send(addresses, subject, body); status != http.StatusOK {
		a.logger(ctx).Errorw(
			"error sending email",
			"email", addresses,
			"subject", subject,
			"status", status,
			"message", details,
		)
		return false
	}
	return true
}

// find and validate the token
//
// The token's userID field is added to the context's logger.
func (a *Api) token(res http.ResponseWriter, req *http.Request) *shoreline.TokenData {
	ctx := req.Context()
	if token := req.Header.Get(TP_SESSION_TOKEN); token != "" {
		td := a.sl.CheckToken(token)

		if td == nil {
			a.sendError(ctx, res, http.StatusForbidden, STATUS_INVALID_TOKEN,
				zap.String("token", token))
			return nil
		}
		//all good!

		ctxLog := a.logger(ctx).With(zap.String("token's userID", td.UserID))
		if td.IsServer {
			ctxLog = a.logger(ctx).With(zap.String("token's userID", "<server>"))
		}
		*req = *req.WithContext(context.WithValue(ctx, ctxLoggerKey{}, ctxLog))

		return td
	}
	a.sendError(ctx, res, http.StatusUnauthorized, STATUS_NO_TOKEN)
	return nil
}

// send metric
func (a *Api) logMetric(name string, req *http.Request) {
	token := req.Header.Get(TP_SESSION_TOKEN)
	emptyParams := make(map[string]string)
	a.metrics.PostThisUser(name, token, emptyParams)
}

// send metric
func (a *Api) logMetricAsServer(name string) {
	token := a.sl.TokenProvide()
	emptyParams := make(map[string]string)
	a.metrics.PostServer(name, token, emptyParams)
}

// Find existing user based on the given indentifier
// The indentifier could be either an id or email address
func (a *Api) findExistingUser(ctx context.Context, indentifier, token string) *shoreline.UserData {
	if usr, err := a.sl.GetUser(indentifier, token); err != nil {
		a.logger(ctx).With(zap.Error(err)).Error("getting existing user details")
		return nil
	} else {
		return usr
	}
}

// Makesure we have set the userId on these confirmations
func (a *Api) ensureIdSet(ctx context.Context, userId string, confirmations []*models.Confirmation) {

	if len(confirmations) < 1 {
		return
	}
	for i := range confirmations {
		//set the userid if not set already
		if confirmations[i].UserId == "" {
			a.logger(ctx).Debug("UserId wasn't set for invite so setting it")
			confirmations[i].UserId = userId
			if err := a.Store.UpsertConfirmation(ctx, confirmations[i]); err != nil {
				a.logger(ctx).With(zap.Error(err)).Warn("upserting confirmation")
			}
		}
	}
}

// Populate restrictions
func (a *Api) populateRestrictions(ctx context.Context, user shoreline.UserData, td shoreline.TokenData, confirmations []*models.Confirmation) error {
	for _, conf := range confirmations {
		// Only clinic invites can have restrictions
		if conf.ClinicId != "" {
			resp, err := a.clinics.ListMembershipRestrictionsWithResponse(ctx, conf.ClinicId)
			if err != nil {
				return err
			}
			if resp.StatusCode() != http.StatusOK {
				return fmt.Errorf("unexpected response code %v when fetching membership restrctions from clinic %v", resp.StatusCode(), conf.ClinicId)
			}
			// If a clinic has no restrictions the invite can be accepted
			conf.Restrictions = &models.Restrictions{
				CanAccept: true,
			}
			if resp.JSON200 != nil && resp.JSON200.Restrictions != nil && len(*resp.JSON200.Restrictions) > 0 {
				// The clinic has configured restrictions, the user must match at least one
				conf.Restrictions.CanAccept = false

				for _, restriction := range *resp.JSON200.Restrictions {
					if strings.HasSuffix(strings.ToLower(user.Username), strings.ToLower(fmt.Sprintf("@%s", restriction.EmailDomain))) {
						if restriction.RequiredIdp == nil || *restriction.RequiredIdp == "" {
							// The user's email matches the domain and no required idp is set
							conf.Restrictions.CanAccept = true
							break
						}

						// The user's email matches the domain, it must also match the required IDP
						if strings.ToLower(*restriction.RequiredIdp) == strings.ToLower(td.IdentityProvider) {
							// The invite can be accepted, because the user is already authenticated
							// against the required IDP
							conf.Restrictions.CanAccept = true
						} else {
							// Add the required IDP as a precondition to accepting the invite
							conf.Restrictions.RequiredIdp = *restriction.RequiredIdp
						}

						break
					}
				}
			}
		}
	}

	return nil
}

func (a *Api) sendModelAsResWithStatus(ctx context.Context, res http.ResponseWriter, model interface{}, statusCode int) {
	if jsonDetails, err := json.Marshal(model); err != nil {
		a.logger(ctx).With("model", model, zap.Error(err)).Errorf("trying to send model")
		http.Error(res, "Error marshaling data for response", http.StatusInternalServerError)
	} else {
		res.Header().Set("content-type", "application/json")
		res.WriteHeader(statusCode)
		res.Write(jsonDetails)
	}
}

func (a *Api) sendError(ctx context.Context, res http.ResponseWriter, statusCode int, reason string, extras ...interface{}) {
	a.sendErrorLog(ctx, statusCode, reason, extras...)
	a.sendModelAsResWithStatus(ctx, res, status.NewStatus(statusCode, reason), statusCode)
}

func (a *Api) sendErrorWithCode(ctx context.Context, res http.ResponseWriter, statusCode int, errorCode int, reason string, extras ...interface{}) {
	a.sendErrorLog(ctx, statusCode, reason, extras...)
	a.sendModelAsResWithStatus(ctx, res, status.NewStatusWithError(statusCode, errorCode, reason), statusCode)
}

func (a *Api) sendErrorLog(ctx context.Context, code int, reason string, extras ...interface{}) {
	details := splitExtrasAndErrorsAndFields(extras)
	log := a.logger(ctx).WithOptions(zap.AddCallerSkip(2)).
		Desugar().With(details.Fields...).Sugar().
		With(zap.Int("code", code))
	if len(details.NonErrors) > 0 {
		log = log.With(zap.Array("extras", zapArrayAny(details.NonErrors)))
	}
	if len(details.Errors) == 1 {
		log = log.With(zap.Error(details.Errors[0]))
	} else if len(details.Errors) > 1 {
		log = log.With(zap.Errors("errors", details.Errors))
	}
	if code < http.StatusInternalServerError || len(details.Errors) == 0 {
		// if there are no errors, use info to skip the stack trace, as it's
		// probably not useful
		log.Info(reason)
	} else {
		log.Error(reason)
	}
}

// sendOK helps send a 200 response with a standard form and optional message.
func (a *Api) sendOK(ctx context.Context, res http.ResponseWriter, reason string) {
	a.sendModelAsResWithStatus(ctx, res, status.NewStatus(http.StatusOK, reason), http.StatusOK)
}

type extrasDetails struct {
	Errors    []error
	NonErrors []interface{}
	Fields    []zap.Field
}

func splitExtrasAndErrorsAndFields(extras []interface{}) extrasDetails {
	details := extrasDetails{
		Errors:    []error{},
		NonErrors: []interface{}{},
		Fields:    []zap.Field{},
	}
	for _, extra := range extras {
		if err, ok := extra.(error); ok {
			if err != nil {
				details.Errors = append(details.Errors, err)
			}
		} else if field, ok := extra.(zap.Field); ok {
			details.Fields = append(details.Fields, field)
		} else if extraErrs, ok := extra.([]error); ok {
			if len(extraErrs) > 0 {
				details.Errors = append(details.Errors, extraErrs...)
			}
		} else {
			details.NonErrors = append(details.NonErrors, extra)
		}
	}
	return details
}

func (a *Api) tokenUserHasRequestedPermissions(tokenData *shoreline.TokenData, groupId string, requestedPermissions commonClients.Permissions) (commonClients.Permissions, error) {
	if tokenData.IsServer {
		return requestedPermissions, nil
	} else if tokenData.UserID == groupId {
		return requestedPermissions, nil
	} else if actualPermissions, err := a.gatekeeper.UserInGroup(tokenData.UserID, groupId); err != nil {
		return commonClients.Permissions{}, err
	} else {
		finalPermissions := make(commonClients.Permissions)
		for permission := range requestedPermissions {
			if reflect.DeepEqual(requestedPermissions[permission], actualPermissions[permission]) {
				finalPermissions[permission] = requestedPermissions[permission]
			}
		}
		return finalPermissions, nil
	}
}

// zapArrayAny helps convert extras to strings for inclusion in a structured
// log message.
func zapArrayAny(extras []interface{}) zapcore.ArrayMarshalerFunc {
	return zapcore.ArrayMarshalerFunc(func(enc zapcore.ArrayEncoder) error {
		for _, extra := range extras {
			enc.AppendString(fmt.Sprintf("%v", extra))
		}
		return nil
	})
}
