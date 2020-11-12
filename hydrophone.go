package main

import (
	"context"
	"net/http"

	"github.com/tidepool-org/go-common/clients"
	ev "github.com/tidepool-org/go-common/events"
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

//InvocationParams are the parameters need to kick off a service
type InvocationParams struct {
	fx.In
	Lifecycle      fx.Lifecycle
	Shoreline      shoreline.Client
	Config         configuration.InboundConfig
	Server         *common.Server
	EventsConsumer ev.EventConsumer
}

func startService(p InvocationParams) {
	p.Lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {

				if err := p.Shoreline.Start(ctx); err != nil {
					return err
				}

				var start func() error
				if p.Config.Protocol == "https" {
					start = func() error { return p.Server.ListenAndServeTLS(p.Config.SslCertFile, p.Config.SslKeyFile) }
				} else {
					start = func() error { return p.Server.ListenAndServe() }
				}
				if err := start(); err != nil {
					return err
				}

				return nil
			},
			OnStop: func(ctx context.Context) error {
				return p.Server.Close()
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
		fx.Invoke(tracing.StartTracer, cloudevents.StartEventConsumer, startService),
	).Run()
}
