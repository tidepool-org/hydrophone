package clients

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	// CharSet The character encoding for the email.
	CharSet = "UTF-8"

	// DefaultTextMessage will be sent to non-HTML email clients that receive our messages
	DefaultTextMessage = "You need an HTML client to read this email."
)

type (
	// SesNotifier contains all information needed to send Amazon SES messages
	SesNotifier struct {
		Config *SesNotifierConfig
		SES    *ses.SES
		log    *zap.SugaredLogger
	}

	// SesNotifierConfig contains the static configuration for the Amazon SES service
	// Credentials come from the environment and are not passed in via configuration variables.
	SesNotifierConfig struct {
		UseMockNotifier bool   `envconfig:"HYDROPHONE_USE_MOCK_NOTIFIER" default:"false"`
		FromAddress     string `split_words:"true" default:"Tidepool <noreply@tidepool.org>"`
		Region          string `default:"us-west-2"`
	}
)

func notifierConfigProvider() (SesNotifierConfig, error) {
	var config SesNotifierConfig
	err := envconfig.Process("ses", &config)
	if err != nil {
		return SesNotifierConfig{}, err
	}
	return config, nil
}

func sesNotifierProvider(config SesNotifierConfig, log *zap.SugaredLogger) (Notifier, error) {
	if config.UseMockNotifier {
		return NewMockNotifier(), nil
	}
	mail, err := NewSesNotifier(&config, log)
	return mail, err
}

// SesModule is a fx module for this component
var SesModule = fx.Options(
	fx.Provide(sesNotifierProvider),
	fx.Provide(notifierConfigProvider),
)

// NewSesNotifier creates a new Amazon SES notifier
func NewSesNotifier(cfg *SesNotifierConfig, log *zap.SugaredLogger) (*SesNotifier, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(cfg.Region)},
	)

	if err != nil {
		return nil, err
	}

	return &SesNotifier{
		Config: cfg,
		SES:    ses.New(sess),
		log:    log,
	}, nil
}

// Send a message to a list of recipients with a given subject
func (c *SesNotifier) Send(to []string, subject string, msg string) (int, string) {
	var toAwsAddress = make([]*string, len(to))
	for i, x := range to {
		toAwsAddress[i] = aws.String(x)
	}

	input := &ses.SendEmailInput{
		Destination: &ses.Destination{
			CcAddresses: []*string{},
			ToAddresses: toAwsAddress,
		},
		Message: &ses.Message{
			Body: &ses.Body{
				Html: &ses.Content{
					Charset: aws.String(CharSet),
					Data:    aws.String(msg),
				},
				Text: &ses.Content{
					Charset: aws.String(CharSet),
					Data:    aws.String(DefaultTextMessage),
				},
			},
			Subject: &ses.Content{
				Charset: aws.String(CharSet),
				Data:    aws.String(subject),
			},
		},
		Source: aws.String(c.Config.FromAddress),
	}

	// Attempt to send the email.
	result, err := c.SES.SendEmail(input)

	// Display error messages if they occur.
	if err != nil {
		c.log.With(zap.Error(err)).Error("sending email")
		return 400, result.String()
	}
	return 200, result.String()
}
