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
	Config struct {
		clients.Config
		Service disc.ServiceListing  `json:"service"`
		Mongo   mongo.Config         `json:"mongo"`
		Api     api.Config           `json:"hydrophone"`
		Mail    sc.SesNotifierConfig `json:"sesEmail"`
	}
)

func main() {
	var config Config

	if err := common.LoadEnvironmentConfig([]string{"TIDEPOOL_HYDROPHONE_ENV", "TIDEPOOL_HYDROPHONE_SERVICE"}, &config); err != nil {
		log.Panic("Problem loading config ", err)
	}

	/*
	 * Hakken setup
	 */
	hakkenClient := hakken.NewHakkenBuilder().
		WithConfig(&config.HakkenConfig).
		Build()

	if err := hakkenClient.Start(); err != nil {
		log.Fatal(err)
	}
	defer hakkenClient.Close()

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

	gatekeeper := clients.NewGatekeeperClientBuilder().
		WithHostGetter(config.GatekeeperConfig.ToHostGetter(hakkenClient)).
		WithHttpClient(httpClient).
		WithSecret(config.ShorelineConfig.ShorelineClientConfig.Secret).
		Build()

	highwater := highwater.NewHighwaterClientBuilder().
		WithHostGetter(config.HighwaterConfig.ToHostGetter(hakkenClient)).
		WithHttpClient(httpClient).
		WithConfig(&config.HighwaterConfig.HighwaterClientConfig).
		WithSecret(config.ShorelineConfig.ShorelineClientConfig.Secret).
		Build()

	seagull := clients.NewSeagullClientBuilder().
		WithHostGetter(config.SeagullConfig.ToHostGetter(hakkenClient)).
		WithHttpClient(httpClient).
		WithSecret(config.ShorelineConfig.ShorelineClientConfig.Secret).
		Build()

	/*
	 * hydrophone setup
	 */
	store := sc.NewMongoStoreClient(&config.Mongo)
	mail := sc.NewSesNotifier(&config.Mail)

	emailTemplates, err := templates.New()
	if err != nil {
		log.Fatal(err)
	}

	rtr := mux.NewRouter()
	api := api.InitApi(config.Api, store, mail, shoreline, gatekeeper, highwater, seagull, emailTemplates)
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
