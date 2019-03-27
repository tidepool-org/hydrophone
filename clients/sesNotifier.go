package clients

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
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
	}

	// SesNotifierConfig contains the static configuration for the Amazon SES service
	// Credentials come from the environment and are not passed in via configuration variables.
	SesNotifierConfig struct {
		From   string `json:"fromAddress"`
		Region string `json:"region"`
	}
)

//NewSesNotifier creates a new Amazon SES notifier
func NewSesNotifier(cfg *SesNotifierConfig) (*SesNotifier, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(cfg.Region)},
	)

	if err != nil {
		return nil, err
	}

	return &SesNotifier{
		Config: cfg,
		SES:    ses.New(sess),
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
		Source: aws.String(c.Config.From),
	}

	// Attempt to send the email.
	result, err := c.SES.SendEmail(input)

	// Display error messages if they occur.
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ses.ErrCodeMessageRejected:
				log.Printf("%v: %v\n", ses.ErrCodeMessageRejected, aerr.Error())
			case ses.ErrCodeMailFromDomainNotVerifiedException:
				log.Printf("%v: %v\n", ses.ErrCodeMailFromDomainNotVerifiedException, aerr.Error())
			case ses.ErrCodeConfigurationSetDoesNotExistException:
				log.Printf("%v: %v\n", ses.ErrCodeConfigurationSetDoesNotExistException, aerr.Error())
			default:
				log.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			log.Println(err.Error())
		}

		return 500, result.String()
	}
	return 200, result.String()
}
