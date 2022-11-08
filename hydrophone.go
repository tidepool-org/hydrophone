// @title Hydrophone API
// @version 1.5.0
// @description The purpose of this API is to send notifications to users: forgotten passwords, initial signup, invitations and more
// @license.name BSD 2-Clause "Simplified" License
// @host api.android-qa.your-loops.dev
// @BasePath /confirm
// @accept json
// @produce json
// @schemes https

// @securityDefinitions.apikey TidepoolAuth
// @in header
// @name x-tidepool-session-token

package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mdblp/go-common/v2/clients/auth"
	"github.com/mdblp/go-common/v2/clients/portal"
	"github.com/mdblp/go-common/v2/clients/version"
	seagullClient "github.com/mdblp/seagull/client"

	"github.com/gorilla/mux"

	crewClient "github.com/mdblp/crew/client"
	common "github.com/mdblp/go-common"
	"github.com/mdblp/go-db/mongo"
	"github.com/mdblp/hydrophone/api"
	sc "github.com/mdblp/hydrophone/clients"
	"github.com/mdblp/hydrophone/localize"
	"github.com/mdblp/hydrophone/templates"
	"github.com/mdblp/shoreline/clients/shoreline"
)

type (
	// Config is the configuration for the service
	Config struct {
		// Mongo        mongo.Config          `json:"mongo"`
		Api          api.Config            `json:"hydrophone"`
		Ses          sc.SesNotifierConfig  `json:"sesEmail"`
		Smtp         sc.SmtpNotifierConfig `json:"smtpEmail"`
		NotifierType string                `json:"notifierType"`
	}
)

func main() {
	var config Config
	logger := log.New()
	logger.Out = os.Stdout
	logger.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	logger.SetReportCaller(true)
	envLogLevel := os.Getenv("LOG_LEVEL")
	logLevel, err := log.ParseLevel(envLogLevel)
	if err != nil {
		logLevel = log.WarnLevel
	}
	logger.SetLevel(logLevel)
	logger.Printf("Starting hydophone service %v\n", version.GetVersion().String())

	servicePort := os.Getenv("HYDROPHONE_PORT")
	if servicePort == "" {
		servicePort = "9157"
	}
	// Load configuration from environment variables
	if err := common.LoadEnvironmentConfig([]string{"TIDEPOOL_HYDROPHONE_SERVICE"}, &config); err != nil {
		logger.Panic("Problem loading config ", err)
	}
	isTestEnv, found := os.LookupEnv("TEST")
	if found && strings.ToUpper(isTestEnv) == "TRUE" {
		config.Api.EnableTestRoutes = true
	} else {
		config.Api.EnableTestRoutes = false
	}
	region, found := os.LookupEnv("REGION")
	if found {
		config.Ses.Region = region
	}

	if config.Ses.Region == "" {
		config.Ses.Region = "us-west-2"
	}

	// server secret may be passed via a separate env variable to accomodate easy secrets injection via Kubernetes
	serverSecret, found := os.LookupEnv("SERVER_SECRET")
	if found {
		config.Api.ServerSecret = serverSecret
	}
	authApiSecret, found := os.LookupEnv("AUTH_SECRET")
	if !found {
		logger.Panic("AUTH_SECRET env var is not found")
	}

	authClient, _ := auth.NewClient(authApiSecret)

	protocol, found := os.LookupEnv("PROTOCOL")
	if found {
		config.Api.Protocol = protocol
	} else {
		config.Api.Protocol = "https"
	}

	if config.Api.ConfirmationAttempts == 0 {
		attempts, _ := os.LookupEnv("CONFIRMATION_ATTEMPTS")
		config.Api.ConfirmationAttempts, _ = strconv.ParseInt(attempts, 10, 64)
		if config.Api.ConfirmationAttempts == 0 {
			config.Api.ConfirmationAttempts = 10
		}
	}
	if config.Api.ConfirmationAttemptsTimeWindow == 0 {
		attemptsWindow, _ := os.LookupEnv("CONFIRMATION_ATTEMPTS_TIME_WINDOW")
		config.Api.ConfirmationAttemptsTimeWindow, _ = time.ParseDuration(attemptsWindow)
		if config.Api.ConfirmationAttemptsTimeWindow == 0 {
			config.Api.ConfirmationAttemptsTimeWindow = time.Hour
		}
	}

	/*
	 * Clients
	 */

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	httpClient := &http.Client{Transport: tr}

	shoreline := shoreline.NewShorelineClientFromEnv(httpClient)

	if err := shoreline.Start(); err != nil {
		logger.Fatal(err)
	}

	logger.Print("Shoreline client started")

	permsClient := crewClient.NewCrewApiClientFromEnv(httpClient)
	seagull, err := seagullClient.NewClientFromEnv(httpClient)
	if err != nil {
		logger.Fatal(err)
	}

	portal, err := portal.NewClientFromEnv(httpClient)
	if err != nil {
		logger.Fatal(err)
	}
	/*
	* hydrophone setup
	 */
	var mongoConfig mongo.Config
	mongoConfig.FromEnv()
	store, err := sc.NewStore(&mongoConfig, logger)
	/* Check that database configuration is valid. It does not check database availability */
	if err != nil {
		logger.Fatal(err)
	}
	defer store.Close()
	store.Start()
	// Create a notifier based on configuration
	var mail sc.Notifier
	var mailErr error
	// defaults the mail exchange service to ses
	if config.NotifierType == "" {
		config.NotifierType = "ses"
	}
	switch config.NotifierType {
	case "ses":
		mail, mailErr = sc.NewSesNotifier(&config.Ses)
	case "smtp":
		mail, mailErr = sc.NewSmtpNotifier(&config.Smtp)
	case "null":
		mail, mailErr = sc.NewNullNotifier()
	default:
		logger.Fatalf("the mail system provided in the configuration (%s) is invalid", config.NotifierType)
	}
	if mailErr != nil {
		logger.Fatal(mailErr)
	} else {
		logger.Printf("Mail client %s created", config.NotifierType)
	}

	// Create a localizer to be used by the templates
	localizer, err := localize.NewI18nLocalizer(path.Join(config.Api.I18nTemplatesPath, "/locales"))
	if err != nil {
		logger.Fatalf("Problem creating i18n localizer %s", err)
	}
	// Create collection of pre-compiled templates
	// Templates are built based on HTML files which location is calculated from config
	// Config is initalized with environment variables
	emailTemplates, err := templates.New(config.Api.I18nTemplatesPath, localizer)
	if err != nil {
		logger.Fatal(err)
	}

	rtr := mux.NewRouter()
	api := api.InitApi(config.Api, store, mail, shoreline, permsClient, authClient, seagull, portal, emailTemplates, logger)
	api.SetHandlers("", rtr)

	/*
	 * Serve it up and publish
	 */
	logger.Printf("Creating http server on 0.0.0.0:%s", servicePort)
	srv := &http.Server{
		Addr:    ":" + servicePort,
		Handler: rtr,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Println("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown:", err)
	}

	logger.Println("Server exiting")

}
