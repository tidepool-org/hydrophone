package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"

	common "github.com/tidepool-org/go-common"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/disc"
	"github.com/tidepool-org/go-common/clients/hakken"
	"github.com/tidepool-org/go-common/clients/highwater"
	"github.com/tidepool-org/go-common/clients/mongo"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/hydrophone/api"
	sc "github.com/tidepool-org/hydrophone/clients"
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

	// Load configuration from environment variables
	if err := common.LoadEnvironmentConfig([]string{"TIDEPOOL_HYDROPHONE_ENV", "TIDEPOOL_HYDROPHONE_SERVICE"}, &config); err != nil {
		log.Panic("Problem loading config ", err)
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
		config.ShorelineConfig.Secret = serverSecret
		config.Api.ServerSecret = serverSecret
	}
	/*
	 * Hakken setup
	 */
	hakkenClient := hakken.NewHakkenBuilder().
		WithConfig(&config.HakkenConfig).
		Build()

	if !config.HakkenConfig.SkipHakken {
		if err := hakkenClient.Start(); err != nil {
			log.Fatal(err)
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
		log.Fatal(err)
	}

	log.Printf("Shoreline client started with server token %s", shoreline.TokenProvide())

	gatekeeper := clients.NewGatekeeperClientBuilder().
		WithHostGetter(config.GatekeeperConfig.ToHostGetter(hakkenClient)).
		WithHttpClient(httpClient).
		WithTokenProvider(shoreline).
		Build()

	highwater := highwater.NewHighwaterClientBuilder().
		WithHostGetter(config.HighwaterConfig.ToHostGetter(hakkenClient)).
		WithHttpClient(httpClient).
		WithConfig(&config.HighwaterConfig.HighwaterClientConfig).
		Build()

	seagull := clients.NewSeagullClientBuilder().
		WithHostGetter(config.SeagullConfig.ToHostGetter(hakkenClient)).
		WithHttpClient(httpClient).
		Build()

		/*
		 * hydrophone setup
		 */
	store := sc.NewMongoStoreClient(&config.Mongo)

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
	default:
		log.Fatalf("the mail system provided in the configuration (%s) is invalid", config.NotifierType)
	}
	if mailErr != nil {
		log.Fatal(mailErr)
	} else {
		log.Printf("Mail client %s created", config.NotifierType)
	}

	// Create collection of pre-compiled templates
	// Templates are built based on HTML files which location is calculated from config
	// Config is initalized with environment variables
	emailTemplates, err := templates.New(config.Api.I18nTemplatesPath)
	if err != nil {
		log.Fatal(err)
	}

	rtr := mux.NewRouter()
	api := api.InitApiWithI18n(config.Api, store, mail, shoreline, gatekeeper, highwater, seagull, emailTemplates)
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
		log.Fatal(err)
	}

	hakkenClient.Publish(&config.Service)

	signals := make(chan os.Signal, 40)
	signal.Notify(signals)
	go func() {
		for {
			sig := <-signals
			log.Printf("Got signal [%s]", sig)

			if sig == syscall.SIGINT || sig == syscall.SIGTERM {
				server.Close()
				done <- true
			}
		}
	}()

	<-done

}
