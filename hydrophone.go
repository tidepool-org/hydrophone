package main

import (
	"context"
	"net/http"

	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/tracing"
	"github.com/tidepool-org/hydrophone/events"

	"github.com/gorilla/mux"
	"go.uber.org/fx"

	common "github.com/tidepool-org/go-common"
	"github.com/tidepool-org/go-common/clients/configuration"
	"github.com/tidepool-org/go-common/clients/highwater"
	"github.com/tidepool-org/go-common/clients/shoreline"
	cloudevents "github.com/tidepool-org/go-common/events"
	"github.com/tidepool-org/hydrophone/api"
	sc "github.com/tidepool-org/hydrophone/clients"
	"github.com/tidepool-org/hydrophone/models"
	"github.com/tidepool-org/hydrophone/templates"
)

func emailTemplateProvider() (models.Templates, error) {
	emailTemplates, err := templates.New()
	return emailTemplates, err
}

func serverProvider(config configuration.InboundConfig, rtr *mux.Router) *common.Server {
	return common.NewServer(&http.Server{
		Addr:    config.ListenAddress,
		Handler: rtr,
	})
}

func startShoreline(shoreline shoreline.Client, lifecycle fx.Lifecycle) {
	lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				if err := shoreline.Start(ctx); err != nil {
					return err
				}
				return nil
			},
			OnStop: func(ctx context.Context) error {
				shoreline.Close(ctx)
				return nil
			},
		},
	)
}

func startService(server *common.Server, config configuration.InboundConfig, lifecycle fx.Lifecycle) {
	lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				var start func() error
				if config.Protocol == "https" {
					start = func() error { return server.ListenAndServeTLS(config.SslCertFile, config.SslKeyFile) }
				} else {
					start = func() error { return server.ListenAndServe() }
				}
				if err := start(); err != nil {
					return err
				}

				return nil
			},
			OnStop: func(ctx context.Context) error {
				return server.Close()
			},
		},
	)
}

func main() {
	fx.New(
		sc.SesModule,
		sc.MongoModule,
		api.RouterModule,
		tracing.TracingModule,
		fx.Provide(
			cloudevents.CloudEventsConfigProvider,
			cloudevents.CloudEventsConsumerProvider,
			events.NewHandler,
		),
		clients.SeagullModule,
		clients.GatekeeperModule,
		shoreline.ShorelineModule,
		highwater.HighwaterModule,
		configuration.Module,
		fx.Provide(
			emailTemplateProvider,
			serverProvider,
			api.NewApi,
		),
		fx.Invoke(tracing.StartTracer, cloudevents.StartEventConsumer, startShoreline, startService),
	).Run()
}
