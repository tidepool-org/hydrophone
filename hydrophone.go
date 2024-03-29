package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/fx"
	"go.uber.org/zap"

	clinicsClient "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/disc"
	"github.com/tidepool-org/go-common/clients/highwater"
	"github.com/tidepool-org/go-common/clients/shoreline"
	ev "github.com/tidepool-org/go-common/events"
	"github.com/tidepool-org/hydrophone/api"
	sc "github.com/tidepool-org/hydrophone/clients"
	"github.com/tidepool-org/hydrophone/events"
	"github.com/tidepool-org/hydrophone/models"
	"github.com/tidepool-org/hydrophone/templates"
	"github.com/tidepool-org/platform/alerts"
	"github.com/tidepool-org/platform/auth"
	authclient "github.com/tidepool-org/platform/auth/client"
	"github.com/tidepool-org/platform/client"
	platformlog "github.com/tidepool-org/platform/log"
	"github.com/tidepool-org/platform/platform"
)

var defaultStopTimeout = 60 * time.Second

type (
	// OutboundConfig contains how to communicate with the dependent services
	OutboundConfig struct {
		Protocol                string `default:"https"`
		ServerSecret            string `split_words:"true" required:"true"`
		AuthClientAddress       string `split_words:"true" required:"true"`
		PermissionClientAddress string `split_words:"true" required:"true"`
		MetricsClientAddress    string `split_words:"true" required:"true"`
		SeagullClientAddress    string `split_words:"true" required:"true"`
		ClinicClientAddress     string `split_words:"true" required:"true"`
		DataClientAddress       string `split_words:"true" required:"true"`
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
	return shoreline.NewShorelineClientBuilder().
		WithHostGetter(disc.NewStaticHostGetterFromString(config.AuthClientAddress)).
		WithHttpClient(httpClient).
		WithName("hydrophone").
		WithSecret(config.ServerSecret).
		WithTokenRefreshInterval(time.Hour).
		Build()
}

func gatekeeperProvider(config OutboundConfig, shoreline shoreline.Client, httpClient *http.Client) clients.Gatekeeper {
	return clients.NewGatekeeperClientBuilder().
		WithHostGetter(disc.NewStaticHostGetterFromString(config.PermissionClientAddress)).
		WithHttpClient(httpClient).
		WithTokenProvider(shoreline).
		Build()
}

func highwaterProvider(config OutboundConfig, httpClient *http.Client) highwater.Client {
	return highwater.NewHighwaterClientBuilder().
		WithHostGetter(disc.NewStaticHostGetterFromString(config.MetricsClientAddress)).
		WithHttpClient(httpClient).
		WithName("highwater").
		WithSource("hydrophone").
		WithVersion("v0.0.1").
		Build()
}

func seagullProvider(config OutboundConfig, httpClient *http.Client) clients.Seagull {
	return clients.NewSeagullClientBuilder().
		WithHostGetter(disc.NewStaticHostGetterFromString(config.SeagullClientAddress)).
		WithHttpClient(httpClient).
		Build()
}

func alertsProvider(config OutboundConfig, tokenProvider auth.ExternalAccessor, logger platformlog.Logger) (api.AlertsClient, error) {
	cfg := client.NewConfig()
	cfg.Address = config.DataClientAddress
	platformCfg := platform.NewConfig()
	platformCfg.Config = cfg
	platformClient, err := platform.NewClient(platformCfg, platform.AuthorizeAsService)
	if err != nil {
		return nil, err
	}
	return alerts.NewClient(platformClient, tokenProvider, logger), nil
}

func zapPlatformAdapterProvider(zapper *zap.SugaredLogger) platformlog.Logger {
	return NewZapPlatformAdapter(zapper)
}

func externalConfigLoaderProvider(loader platform.ConfigLoader) authclient.ExternalConfigLoader {
	return authclient.NewExternalEnvconfigLoader(loader)
}

func platformConfigLoaderProvider(loader client.ConfigLoader) platform.ConfigLoader {
	return platform.NewEnvconfigLoader(loader)
}

func clientConfigLoaderProvider() client.ConfigLoader {
	return client.NewEnvconfigLoader()
}

func clinicProvider(config OutboundConfig, shoreline shoreline.Client) (clinicsClient.ClientWithResponsesInterface, error) {
	opts := clinicsClient.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Add(api.TP_SESSION_TOKEN, shoreline.TokenProvide())
		return nil
	})
	return clinicsClient.NewClientWithResponses(config.ClinicClientAddress, opts)
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

	return &http.Client{Transport: tr}
}

func emailTemplateProvider() (models.Templates, error) {
	emailTemplates, err := templates.New()
	return emailTemplates, err
}

func serverProvider(config InboundConfig, rtr *mux.Router) *http.Server {
	return &http.Server{
		Addr:    config.ListenAddress,
		Handler: rtr,
	}
}

func cloudEventsConfigProvider() (*ev.CloudEventsConfig, error) {
	cfg := ev.NewConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func faultTolerantConsumerProvider(config *ev.CloudEventsConfig, handler ev.EventHandler) (ev.EventConsumer, error) {
	return ev.NewFaultTolerantConsumerGroup(config, func() (ev.MessageConsumer, error) {
		handlers := []ev.EventHandler{handler}
		return ev.NewCloudEventsMessageHandler(handlers)
	})
}

func loggerProvider() (*zap.SugaredLogger, error) {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	config.EncoderConfig.FunctionKey = "function"
	logger, err := config.Build()
	if err != nil {
		return nil, err
	}
	return logger.Sugar(), nil
}

// InvocationParams are the parameters need to kick off a service
type InvocationParams struct {
	fx.In
	Lifecycle  fx.Lifecycle
	Shutdowner fx.Shutdowner
	Shoreline  shoreline.Client
	Config     InboundConfig
	Server     *http.Server
	Consumer   ev.EventConsumer
	Log        *zap.SugaredLogger
}

func startEventConsumer(p InvocationParams) {
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := p.Consumer.Start(); err != nil {
					p.Log.With(zap.Error(err)).Error("starting cloud events consumer")
					p.Log.Infof("shutting down the service")
					if shutdownErr := p.Shutdowner.Shutdown(); shutdownErr != nil {
						p.Log.With(zap.Error(shutdownErr)).Error("failed to shutdown")
					}
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return p.Consumer.Stop()
		},
	})
}

func startShoreline(p InvocationParams) {
	p.Lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				if err := p.Shoreline.Start(); err != nil {
					p.Log.With(zap.Error(err)).Error("starting shoreline")
					return err
				}
				return nil
			},
			OnStop: func(ctx context.Context) error {
				p.Shoreline.Close()
				return nil
			},
		},
	)
}

func startServer(p InvocationParams) {
	p.Lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				go func() {
					if err := p.Server.ListenAndServe(); err != nil {
						p.Log.With(zap.Error(err)).Error("while listening")
						p.Log.Infof("shutting down")
						if shutdownErr := p.Shutdowner.Shutdown(); shutdownErr != nil {
							p.Log.With(zap.Error(err)).Error("shutting down")
						}
					}
				}()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				return p.Server.Shutdown(ctx)
			},
		},
	)
}

func main() {
	fx.New(
		sc.SesModule,
		sc.MongoModule,
		api.RouterModule,
		fx.Provide(
			cloudEventsConfigProvider,
			faultTolerantConsumerProvider,
			events.NewHandler,
		),
		authclient.ExternalClientModule,
		authclient.ProvideServiceName("hydrophone"),
		fx.Provide(
			externalConfigLoaderProvider,
			platformConfigLoaderProvider,
			clientConfigLoaderProvider,
			zapPlatformAdapterProvider,
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
			clinicProvider,
			loggerProvider,
			alertsProvider,
			api.NewApi,
		),
		fx.Invoke(startShoreline),
		fx.Invoke(startEventConsumer),
		fx.Invoke(startServer),
		fx.StopTimeout(defaultStopTimeout),
	).Run()
}
