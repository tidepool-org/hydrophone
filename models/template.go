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
	}

	Template struct {
		compiled    *template.Template
		Subject     string
		BodyContent string
	}
)

func NewTemplate() *Template {
	return &Template{}
}

/*
 * Load the correct template based on type and returned it compiled
 */
func (t *Template) Load(templateType Type, cfg *TemplateConfig) {

	var compiled *template.Template
	var subject string

	switch {
	case templateType == TypeCareteamInvite:
		compiled = template.Must(template.New(string(TypeCareteamInvite)).Parse(cfg.CareteamInvite))
		subject = cfg.CareteamInviteSubject
		break
	case templateType == TypeSignUp:
		compiled = template.Must(template.New(string(TypeSignUp)).Parse(cfg.Signup))
		subject = cfg.SignupSubject
		break
	case templateType == TypePasswordReset:
		compiled = template.Must(template.New(string(TypePasswordReset)).Parse(cfg.PasswordReset))
		subject = cfg.PasswordResetSubject
		break
	default:
		log.Println("Unknown type ", templateType)
		compiled = nil
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
