package models

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"strconv"
)

type TemplateName string

func (t TemplateName) String() string {
	return string(t)
}

const (
	TemplateNameCareteamInvite        TemplateName = "careteam_invitation"
	TemplateNameNoAccount             TemplateName = "no_account"
	TemplateNamePasswordReset         TemplateName = "password_reset"
	TemplateNamePatientPasswordReset  TemplateName = "patient_password_reset"
	TemplateNameSignup                TemplateName = "signup_confirmation"
	TemplateNameSignupClinic          TemplateName = "signup_clinic_confirmation"
	TemplateNameSignupCustodial       TemplateName = "signup_custodial_confirmation"
	TemplateNameSignupCustodialClinic TemplateName = "signup_custodial_clinic_confirmation"
	TemplateNameTest                  TemplateName = "test_template"
	TemplateNameUndefined             TemplateName = ""
)

type Template interface {
	Name() TemplateName
	Execute(content interface{}) (string, string, error)
	ContentParts() []string
	EscapeParts() []string
	Subject() string
}

type Templates map[TemplateName]Template

type PrecompiledTemplate struct {
	name               TemplateName
	precompiledSubject *template.Template
	precompiledBody    *template.Template
	contentParts       []string
	subject            string
	escapeParts        []string
}

// NewPrecompiledTemplate creates a new pre-compiled template
func NewPrecompiledTemplate(name TemplateName, subjectTemplate string, bodyTemplate string, contentParts []string, escapeParts []string) (*PrecompiledTemplate, error) {
	if name == TemplateNameUndefined {
		return nil, errors.New("models: name is missing")
	}
	if subjectTemplate == "" {
		return nil, errors.New("models: subject template is missing")
	}
	if bodyTemplate == "" {
		return nil, errors.New("models: body template is missing")
	}

	precompiledSubject, err := template.New(name.String()).Parse(subjectTemplate)
	if err != nil {
		return nil, fmt.Errorf("models: failure to precompile subject template: %s", err)
	}

	precompiledBody, err := template.New(name.String()).Parse(bodyTemplate)
	if err != nil {
		return nil, fmt.Errorf("models: failure to precompile body template: %s", err)
	}

	return &PrecompiledTemplate{
		name:               name,
		precompiledSubject: precompiledSubject,
		precompiledBody:    precompiledBody,
		subject:            subjectTemplate,
		contentParts:       contentParts,
		escapeParts:        escapeParts,
	}, nil
}

// Name of the template
func (p *PrecompiledTemplate) Name() TemplateName {
	return p.name
}

// Subject of the template
func (p *PrecompiledTemplate) Subject() string {
	return p.subject
}

// ContentParts returns the content parts of the template
// Content parts are the items that are dynamically localized and added in the html tags
func (p *PrecompiledTemplate) ContentParts() []string {
	return p.contentParts
}

// EscapeParts returns the escape parts of the template
// These parts are those that are not translated with go-i18n but need to be replaced dynamically by Tidepool engine
func (p *PrecompiledTemplate) EscapeParts() []string {
	return p.escapeParts
}

// Execute compiles the pre-compiled template with provided content
func (p *PrecompiledTemplate) Execute(content interface{}) (string, string, error) {

	var subjectBuffer bytes.Buffer
	var bodyBuffer bytes.Buffer

	if err := p.precompiledSubject.Execute(&subjectBuffer, content); err != nil {
		return "", "", fmt.Errorf("models: failure to execute subject template %s with content", strconv.Quote(p.name.String()))
	}

	if err := p.precompiledBody.Execute(&bodyBuffer, content); err != nil {
		return "", "", fmt.Errorf("models: failure to execute body template %s with content", strconv.Quote(p.name.String()))
	}

	return subjectBuffer.String(), bodyBuffer.String(), nil
}
