package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"net/url"
	"time"

	ev "github.com/tidepool-org/go-common/events"
	"github.com/tidepool-org/go-common/tracing"
	"github.com/tidepool-org/hydrophone/events"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/gorilla/mux"
	"go.uber.org/fx"

	"github.com/kelseyhightower/envconfig"
	common "github.com/tidepool-org/go-common"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/highwater"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/hydrophone/api"
	sc "github.com/tidepool-org/hydrophone/clients"
	"github.com/tidepool-org/hydrophone/models"
	"github.com/tidepool-org/hydrophone/templates"
)

type (
	// OutboundConfig contains how to communicate with the dependent services
	OutboundConfig struct {
		Protocol                string `default:"https"`
		ServerSecret            string `split_words:"true" required:"true"`
		AuthClientAddress       string `split_words:"true" required:"true"`
		PermissionClientAddress string `split_words:"true" required:"true"`
		MetricsClientAddress    string `split_words:"true" required:"true"`
		SeagullClientAddress    string `split_words:"true" required:"true"`
	}

	//InboundConfig describes how to receive inbound communication
	InboundConfig struct {
		Protocol      string `default:"http"`
		SslKeyFile    string `split_words:"true" default:""`
		SslCertFile   string `split_words:"true" default:""`
		ListenAddress string `split_words:"true" required:"true"`
	}
)

func shorelineProvider(config OutboundConfig, httpClient *http.Client) shoreline.Client {
	host, _ := url.Parse(config.AuthClientAddress)
	return shoreline.NewShorelineClientBuilder().
		WithHost(host).
		WithHttpClient(httpClient).
		WithName("shoreline").
		WithSecret(config.ServerSecret).
		WithTokenRefreshInterval(time.Hour).
		Build()
}

func gatekeeperProvider(config OutboundConfig, shoreline shoreline.Client, httpClient *http.Client) clients.Gatekeeper {
	host, _ := url.Parse(config.PermissionClientAddress)
	return clients.NewGatekeeperClientBuilder().
		WithHost(host).
		WithHttpClient(httpClient).
		WithTokenProvider(shoreline).
		Build()
}

func highwaterProvider(config OutboundConfig, httpClient *http.Client) highwater.Client {
	host, _ := url.Parse(config.MetricsClientAddress)
	return highwater.NewHighwaterClientBuilder().
		WithHost(host).
		WithHttpClient(httpClient).
		WithName("highwater").
		WithSource("hydrophone").
		WithVersion("v0.0.1").
		Build()
}

func seagullProvider(config OutboundConfig, httpClient *http.Client) clients.Seagull {
	host, _ := url.Parse(config.SeagullClientAddress)
	return clients.NewSeagullClientBuilder().
		WithHost(host).
		WithHttpClient(httpClient).
		Build()
}

func configProvider() (OutboundConfig, error) {
	var config OutboundConfig
	err := envconfig.Process("tidepool", &config)
	if err != nil {
		return OutboundConfig{}, err
	}
	return config, nil
}

func serviceConfigProvider() (InboundConfig, error) {
	var config InboundConfig
	err := envconfig.Process("service", &config)
	if err != nil {
		return InboundConfig{}, err
	}
	return config, nil
}

func httpClientProvider() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &http.Client{Transport: otelhttp.NewTransport(tr)}
}

func emailTemplateProvider() (models.Templates, error) {
	emailTemplates, err := templates.New()
	return emailTemplates, err
}

func serverProvider(config InboundConfig, rtr *mux.Router) *common.Server {
	return common.NewServer(&http.Server{
		Addr:    config.ListenAddress,
		Handler: rtr,
	})
}

func cloudEventsConfigProvider() (*ev.CloudEventsConfig, error) {
	cfg := ev.NewConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func cloudEventsConsumerProvider(config *ev.CloudEventsConfig, handler ev.EventHandler) (ev.EventConsumer, error) {
	consumer, err := ev.NewSaramaCloudEventsConsumer(config)
	if err != nil {
		return nil, err
	}
	consumer.RegisterHandler(handler)
	return consumer, nil
}

//InvocationParams are the parameters need to kick off a service
type InvocationParams struct {
	fx.In
	Lifecycle      fx.Lifecycle
	Shoreline      shoreline.Client
	Config         InboundConfig
	Server         *common.Server
	EventsConsumer ev.EventConsumer
}

func startEventConsumer(consumer ev.EventConsumer, lifecycle fx.Lifecycle) {
	consumerCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{}, 1)
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				// blocks until context is terminated
				err := consumer.Start(consumerCtx)
				if err != nil {
					log.Printf("Unable to start cloud events consumer: %v", err)
				}
				done <- struct{}{}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			<-done
			return nil
		},
	})
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
			cloudEventsConfigProvider,
			cloudEventsConsumerProvider,
			events.NewHandler,
		),
		fx.Provide(
			seagullProvider,
			highwaterProvider,
			gatekeeperProvider,
			shorelineProvider,
			configProvider,
			serviceConfigProvider,
			httpClientProvider,
			emailTemplateProvider,
			serverProvider,
			api.NewApi,
		),
		fx.Invoke(tracing.StartTracer, startEventConsumer, startService),
	).Run()
}
