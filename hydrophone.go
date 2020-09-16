// @title Hydrophone API
// @version 0.0.1
// @description The purpose of this API is to send notifications to users: forgotten passwords, initial signup, invitations and more
// @license.name BSD 2-Clause "Simplified" License
// @host localhost
// @BasePath /confirm
// @accept json
// @produce json
// @schemes https

// @securityDefinitions.apikey TidepoolAuth
// @in header
// @name x-tidepool-session-token

package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/tidepool-org/go-common/clients/portal"

	"github.com/tidepool-org/go-common/clients/version"

	"github.com/gorilla/mux"

	common "github.com/tidepool-org/go-common"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/disc"
	"github.com/tidepool-org/go-common/clients/hakken"
	"github.com/tidepool-org/go-common/clients/mongo"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/hydrophone/api"
	sc "github.com/tidepool-org/hydrophone/clients"
	"github.com/tidepool-org/hydrophone/localize"
	"github.com/tidepool-org/hydrophone/templates"
)

type (
	// Config is the configuration for the service
	Config struct {
		clients.Config
		Service      disc.ServiceListing   `json:"service"`
		Mongo        mongo.Config          `json:"mongo"`
		Api          api.Config            `json:"hydrophone"`
		Ses          sc.SesNotifierConfig  `json:"sesEmail"`
		Smtp         sc.SmtpNotifierConfig `json:"smtpEmail"`
		NotifierType string                `json:"notifierType"`
	}
)

func main() {
	var config Config
	logger := log.New(os.Stdout, api.CONFIRM_API_PREFIX, log.LstdFlags|log.Lshortfile)

	logger.Printf("Starting Hydrophone version %s", version.GetVersion().String())
	// Load configuration from environment variables
	if err := common.LoadEnvironmentConfig([]string{"TIDEPOOL_HYDROPHONE_ENV", "TIDEPOOL_HYDROPHONE_SERVICE"}, &config); err != nil {
		logger.Panic("Problem loading config ", err)
	}

	region, found := os.LookupEnv("REGION")
	if found {
		config.Ses.Region = region
	}

	if config.Ses.Region == "" {
		config.Ses.Region = "us-west-2"
	}

	config.Mongo.FromEnv()

	// server secret may be passed via a separate env variable to accomodate easy secrets injection via Kubernetes
	serverSecret, found := os.LookupEnv("SERVER_SECRET")
	if found {
		config.ShorelineConfig.Secret = serverSecret
		config.Api.ServerSecret = serverSecret
	}

	protocol, found := os.LookupEnv("PROTOCOL")
	if found {
		config.Api.Protocol = protocol
	} else {
		config.Api.Protocol = "https"
	}
	/*
	 * Hakken setup
	 */
	hakkenClient := hakken.NewHakkenBuilder().
		WithConfig(&config.HakkenConfig).
		Build()

	if !config.HakkenConfig.SkipHakken {
		if err := hakkenClient.Start(); err != nil {
			logger.Fatal(err)
		}
		defer hakkenClient.Close()
	}

	/*
	 * Clients
	 */

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	httpClient := &http.Client{Transport: tr}

	shoreline := shoreline.NewShorelineClientBuilder().
		WithHostGetter(config.ShorelineConfig.ToHostGetter(hakkenClient)).
		WithHttpClient(httpClient).
		WithConfig(&config.ShorelineConfig.ShorelineClientConfig).
		Build()

	if err := shoreline.Start(); err != nil {
		logger.Fatal(err)
	}

	logger.Print("Shoreline client started")

	gatekeeper := clients.NewGatekeeperClientBuilder().
		WithHostGetter(config.GatekeeperConfig.ToHostGetter(hakkenClient)).
		WithHttpClient(httpClient).
		WithTokenProvider(shoreline).
		Build()

	seagull := clients.NewSeagullClientBuilder().
		WithHostGetter(config.SeagullConfig.ToHostGetter(hakkenClient)).
		WithHttpClient(httpClient).
		Build()

	portal := portal.NewPortalClientBuilder().
		WithHostGetter(config.PortalConfig.ToHostGetter(hakkenClient)).
		WithHTTPClient(httpClient).
		Build()

		/*
		 * hydrophone setup
		 */
	storage, err := sc.NewStore(&config.Mongo, logger)
	if err != nil {
		logger.Fatal(err)
	}
	defer storage.Close()
	storage.Start()
	// Create a notifier based on configuration
	var mail sc.Notifier
	var mailErr error
	//defaults the mail exchange service to ses
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

	//Create a localizer to be used by the templates
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
	api := api.InitApi(config.Api, storage, mail, shoreline, gatekeeper, seagull, portal, emailTemplates)
	api.SetHandlers("", rtr)

	/*
	 * Serve it up and publish
	 */
	done := make(chan bool)
	server := common.NewServer(&http.Server{
		Addr:    config.Service.GetPort(),
		Handler: rtr,
	})

	var start func() error
	if config.Service.Scheme == "https" {
		sslSpec := config.Service.GetSSLSpec()
		start = func() error { return server.ListenAndServeTLS(sslSpec.CertFile, sslSpec.KeyFile) }
	} else {
		start = func() error { return server.ListenAndServe() }
	}
	if err := start(); err != nil {
		logger.Fatal(err)
	}

	hakkenClient.Publish(&config.Service)

	signals := make(chan os.Signal, 40)
	signal.Notify(signals)
	go func() {
		for {
			sig := <-signals
			logger.Printf("Got signal [%s]", sig)

			if sig == syscall.SIGINT || sig == syscall.SIGTERM {
				storage.Close()
				server.Close()
				done <- true
			}
		}
	}()

	<-done

}
