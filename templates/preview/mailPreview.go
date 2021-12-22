// Preview package: preview emails on a simple website
package main

import (
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/mux"
	common "github.com/mdblp/go-common"

	"github.com/mdblp/hydrophone/localize"
	"github.com/mdblp/hydrophone/templates"
)

type Config struct {
	HttpAddress               string `json:"httpaddr"`
	WebURL                    string `json:"webUrl"`                    // used for link to blip
	SupportURL                string `json:"supportUrl"`                // used for link to support
	AssetURL                  string `json:"assetUrl"`                  // used for location of the images
	I18nTemplatesPath         string `json:"i18nTemplatesPath"`         // where are the templates located?
	AllowPatientResetPassword bool   `json:"allowPatientResetPassword"` // true means that patients can reset their password, false means that only clinicianc can reset their password
	PatientPasswordResetURL   string `json:"patientPasswordResetUrl"`   // URL of the help web site that is used to give instructions to reset password for patients
	LocalizeServiceUrl        string `json:"localizeServiceUrl"`
	LocalizeServiceAuthKey    string `json:"localizeServiceAuthKey"`
}

func main() {
	log.Printf("Starting Hydrophone email preview platform")
	var config Config
	if err := common.LoadEnvironmentConfig([]string{"TIDEPOOL_HYDROPHONE_SERVICE"}, &config); err != nil {
		log.Panic("Problem loading config ", err)
	}

	localizer, err := localize.NewI18nLocalizer(path.Join(config.I18nTemplatesPath, "locales/"))
	if err != nil {
		log.Panic("Problem creating i18n localizer ", err)
	}
	emailTemplates, err := templates.New(config.I18nTemplatesPath, localizer)
	if err != nil {
		log.Fatal(err)
	}
	router := mux.NewRouter()
	localizationManager := &DefaultManager{}
	a := InitApi(config, emailTemplates, localizationManager)
	a.SetHandlers("", router)
	/*
	 * Serve it up and publish
	 */
	done := make(chan bool)
	server := common.NewServer(&http.Server{
		Addr:    config.HttpAddress,
		Handler: router,
	})

	var start func() error
	start = func() error { return server.ListenAndServe() }
	if err := start(); err != nil {
		log.Fatal(err)
	}

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
