package models

import (
	"bytes"
	"html/template"
	"log"
)

type (
	TemplateConfig struct {
		PasswordReset         string `json:"passwordReset"`
		PasswordResetSubject  string `json:"passwordResetSubject"`
		CareteamInvite        string `json:"careteamInvite"`
		CareteamInviteSubject string `json:"careteamInviteSubject"`
		Signup                string `json:"signUp"`
		SignupSubject         string `json:"signUpSubject"`
		NoAccount             string `json:"noAccount"`
		NoAccountSubject      string `json:"noAccountSubject"`
	}

	Template struct {
		compiled    *template.Template
		Subject     string
		BodyContent string
	}

	TemplateName string
)

const (
	TemplateNameUndefined      TemplateName = ""
	TemplateNamePasswordReset  TemplateName = "password_reset"
	TemplateNameCareteamInvite TemplateName = "careteam_invitation"
	TemplateNameSignup         TemplateName = "signup_confirmation"
	TemplateNameNoAccount      TemplateName = "no_account"
)

func (t TemplateName) String() string {
	return string(t)
}

func NewTemplate() *Template {
	return &Template{}
}

/*
 * Load the correct template based on type and returned it compiled
 */
func (t *Template) Load(templateName TemplateName, cfg *TemplateConfig) {

	var compiled *template.Template
	var subject string

	switch templateName {
	case TemplateNameCareteamInvite:
		compiled = template.Must(template.New(templateName.String()).Parse(cfg.CareteamInvite))
		subject = cfg.CareteamInviteSubject
		break
	case TemplateNameSignup:
		compiled = template.Must(template.New(templateName.String()).Parse(cfg.Signup))
		subject = cfg.SignupSubject
		break
	case TemplateNamePasswordReset:
		compiled = template.Must(template.New(templateName.String()).Parse(cfg.PasswordReset))
		subject = cfg.PasswordResetSubject
		break
	case TemplateNameNoAccount:
		compiled = template.Must(template.New(templateName.String()).Parse(cfg.NoAccount))
		subject = cfg.NoAccountSubject
		break
	default:
		log.Println("Unknown template name ", templateName)
		compiled = nil
		subject = ""
		break
	}

	t.compiled = compiled
	t.Subject = subject
}

/*
 * Parse the content into the compiled template
 */
func (t *Template) Parse(content interface{}) {

	if t.compiled == nil {
		log.Println("there is no compiled template")
		return
	}

	var buffer bytes.Buffer

	if err := t.compiled.Execute(&buffer, content); err != nil {
		log.Println("error parsing template ", err)
		return
	}

	t.BodyContent = buffer.String()
}
