package clients

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/smtp"
	"net/textproto"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/fx"
)

// Implements a simple SMTP-based notifier that can be used with local development. To enable:
//  - In hydrophone.go, replace "sc.SesModule" with "sc.SMTPModule".
//  - Set three SMTP environment variables on the hydrophone deployment.
//      kubectl set env deployment/hydrophone SMTP_HOST="<host>" SMTP_USERNAME="<username>" SMTP_PASSWORD="<password>"
//  - The hydrophone pod should automatically terminate and start with the SMTP notifier enabled.

const (
	SMTPFrom = "noreply@tidepool.org"
	SMTPPort = 587
)

type SMTPNotifierConfig struct {
	Host     string
	Username string
	Password string
}

func (s SMTPNotifierConfig) IsValid() bool {
	return s.Host != "" && s.Username != "" && s.Password != ""
}

func (s SMTPNotifierConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, SMTPPort)
}

func (s SMTPNotifierConfig) Auth() smtp.Auth {
	return smtp.PlainAuth("", s.Username, s.Password, s.Host)
}

func smtpNotifierConfigProvider() (SMTPNotifierConfig, error) {
	var config SMTPNotifierConfig
	if err := envconfig.Process("SMTP", &config); err != nil {
		return SMTPNotifierConfig{}, err
	}
	return config, nil
}

type SMTPNotifier struct {
	config SMTPNotifierConfig
}

func smtpNotifierProvider(config SMTPNotifierConfig) (Notifier, error) {
	return &SMTPNotifier{
		config: config,
	}, nil
}

func (s *SMTPNotifier) Send(to []string, subject string, message string) (int, string) {
	if len(to) < 1 {
		return http.StatusBadRequest, "to is missing"
	} else if subject == "" {
		return http.StatusBadRequest, "subject is missing"
	} else if message == "" {
		return http.StatusBadRequest, "message is missing"
	}

	if !s.config.IsValid() {
		return http.StatusNotImplemented, "config is invalid"
	}

	encodedMessage, err := s.encodeMessage(to, subject, message)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	if err := smtp.SendMail(s.config.Address(), s.config.Auth(), SMTPFrom, to, encodedMessage); err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	return http.StatusOK, ""
}

func (s *SMTPNotifier) encodeMessage(to []string, subject string, message string) ([]byte, error) {
	messageBuffer := &bytes.Buffer{}
	messageWriter := multipart.NewWriter(messageBuffer)

	fmt.Fprintf(messageBuffer, "To: %s\n", strings.Join(to, ", "))
	fmt.Fprintf(messageBuffer, "Subject: %s\n", subject)
	fmt.Fprintf(messageBuffer, "MIME-Version: 1.0\n")
	fmt.Fprintf(messageBuffer, "Content-Type: multipart/alternative; boundary=\"%s\"\n", messageWriter.Boundary())
	fmt.Fprintf(messageBuffer, "\n")

	textHeaders := textproto.MIMEHeader{}
	textHeaders.Add("Content-Type", "text/plain; charset=UTF-8")
	textHeaders.Add("Content-Transfer-Encoding", "7bit")
	textPart, err := messageWriter.CreatePart(textHeaders)
	if err != nil {
		return nil, err
	}
	if _, err := textPart.Write([]byte(DefaultTextMessage)); err != nil {
		return nil, err
	}

	htmlHeaders := textproto.MIMEHeader{}
	htmlHeaders.Add("Content-Type", "text/html; charset=UTF-8")
	htmlHeaders.Add("Content-Transfer-Encoding", "quoted-printable")
	htmlPart, err := messageWriter.CreatePart(htmlHeaders)
	if err != nil {
		return nil, err
	}

	htmlBuffer := &bytes.Buffer{}
	htmlWriter := quotedprintable.NewWriter(htmlBuffer)
	if _, err := htmlWriter.Write([]byte(message)); err != nil {
		return nil, err
	}
	if _, err := htmlPart.Write(htmlBuffer.Bytes()); err != nil {
		return nil, err
	}

	messageWriter.Close()

	return messageBuffer.Bytes(), nil
}

var SMTPModule = fx.Options(
	fx.Provide(smtpNotifierProvider),
	fx.Provide(smtpNotifierConfigProvider),
)
