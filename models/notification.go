package models

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"html/template"
	"log"
	"strings"
	"time"
)

const (
	PW_RESET        = "password_reset"
	CARETEAM_INVITE = "careteam_invitation"
	CONFIRMATION    = "email_confirmation"
)

type (
	Notification struct {
		Id       string `bson:"_id,omitempty"`
		Key      string
		Content  string
		ToUser   string
		FromUser string    // could be empty
		Created  time.Time //used for expiry
		Sent     time.Time //sent - or maybe failed
	}

	EmailTemplate struct {
		PasswordReset  string `json:"passwordReset"`
		CareteamInvite string `json:"careteamInvite"`
		Confirmation   string `json:"confirmation"`
	}
)

func NewEmailNotification(emailType string, cfg *EmailTemplate, address ...string) (*Notification, error) {

	if key, err := generateKey(emailType); err != nil {
		return nil, err
	} else {

		notification := &Notification{
			Key:     key,
			ToUser:  address[0],
			Created: time.Now(),
		}

		log.Printf("t1 [%v]", notification)

		notification.Content = parseTemplateContent(
			loadTemplate(emailType, cfg),
			notification,
		)

		return notification, nil
	}
}

/*
 * Load the correct compiled template
 */
func loadTemplate(emailType string, cfg *EmailTemplate) *template.Template {

	var compiled *template.Template

	log.Printf("Type [%v] Config [%v]", emailType, cfg)

	switch {
	case strings.Index(strings.ToLower(emailType), CARETEAM_INVITE) != -1:
		log.Printf("CT [%v]", cfg.CareteamInvite)
		compiled = template.Must(template.New(CARETEAM_INVITE).Parse(cfg.CareteamInvite))
		break
	case strings.Index(strings.ToLower(emailType), CONFIRMATION) != -1:
		log.Printf("CONFIRM [%v]", cfg.Confirmation)
		compiled = template.Must(template.New(CONFIRMATION).Parse(cfg.Confirmation))
		break
	case strings.Index(strings.ToLower(emailType), PW_RESET) != -1:
		log.Printf("PW [%v]", cfg.PasswordReset)
		compiled = template.Must(template.New(PW_RESET).Parse(cfg.PasswordReset))
		break
	default:
		log.Println("Unknown type ", emailType)
		compiled = nil
		break
	}

	return compiled
}

/*
 * Parse the content into the template
 */
func parseTemplateContent(compiled *template.Template, content interface{}) string {
	var buffer bytes.Buffer

	if err := compiled.Execute(&buffer, content); err != nil {
		log.Println("error parsing template ", err)
		return ""
	}

	tmpl := buffer.String()

	log.Println("Template: ", tmpl)

	return tmpl
}

/*
 * Generate the unique key used in the URL
 */
func generateKey(emailType string) (string, error) {

	length := 32 // change the length of the generated random string here

	rb := make([]byte, length)
	if _, err := rand.Read(rb); err != nil {
		log.Println(err)
		return "", err
	} else {
		return emailType + "/" + base64.URLEncoding.EncodeToString(rb), nil
	}
}

func (n *Notification) Send() {
	n.Sent = time.Now()
}
