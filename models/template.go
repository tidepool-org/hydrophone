package models

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"strconv"
	textTemplate "text/template"
)

type TemplateName string

func (t TemplateName) String() string {
	return string(t)
}

const (
	TemplateNamePatientClinicInvite TemplateName = "patient_clinic_invitation"
	TemplateNameCareteamInvite      TemplateName = "careteam_invitation"
	// TemplateNameCareteamInviteWithAlerting is sent when a user is invited
	// via the Loop app to use the Care Partner app to receive alert
	// notifications.
	TemplateNameCareteamInviteWithAlerting         TemplateName = "careteam_invitation_with_alerting"
	TemplateNameClinicianInvite                    TemplateName = "clinician_invitation"
	TemplateNameNoAccount                          TemplateName = "no_account"
	TemplateNamePasswordReset                      TemplateName = "password_reset"
	TemplateNameSignup                             TemplateName = "signup_confirmation"
	TemplateNameSignupClinic                       TemplateName = "signup_clinic_confirmation"
	TemplateNameSignupCustodial                    TemplateName = "signup_custodial_confirmation"
	TemplateNameSignupCustodialClinic              TemplateName = "signup_custodial_clinic_confirmation"
	TemplateNameSignupCustodialNewClinicExperience TemplateName = "signup_custodial_new_clinic_experience_confirmation"
	TemplateNameUndefined                          TemplateName = ""
)

type Template interface {
	Name() TemplateName
	Execute(content interface{}) (string, string, error)
}

type Templates map[TemplateName]Template

type PrecompiledTemplate struct {
	name               TemplateName
	precompiledSubject *textTemplate.Template
	precompiledBody    *template.Template
}

func NewPrecompiledTemplate(name TemplateName, subjectTemplate string, bodyTemplate string) (*PrecompiledTemplate, error) {
	if name == TemplateNameUndefined {
		return nil, errors.New("models: name is missing")
	}
	if subjectTemplate == "" {
		return nil, errors.New("models: subject template is missing")
	}
	if bodyTemplate == "" {
		return nil, errors.New("models: body template is missing")
	}

	precompiledSubject, err := textTemplate.New(name.String()).Parse(subjectTemplate)
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
	}, nil
}

func (p *PrecompiledTemplate) Name() TemplateName {
	return p.name
}

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
