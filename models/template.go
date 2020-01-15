package models

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"text/template"

	"github.com/tidepool-org/hydrophone/localize"
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
	TemplateNamePatientInformation    TemplateName = "patient_information"
	TemplateNameSignup                TemplateName = "signup_confirmation"
	TemplateNameSignupClinic          TemplateName = "signup_clinic_confirmation"
	TemplateNameSignupCustodial       TemplateName = "signup_custodial_confirmation"
	TemplateNameSignupCustodialClinic TemplateName = "signup_custodial_clinic_confirmation"
	TemplateNameTest                  TemplateName = "test_template"
	TemplateNameUndefined             TemplateName = ""
)

type Template interface {
	Name() TemplateName
	Execute(content interface{}, lang string) (string, string, error)
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
	localizer          localize.Localizer
}

// NewPrecompiledTemplate creates a new pre-compiled template
func NewPrecompiledTemplate(name TemplateName, subjectTemplate string, bodyTemplate string, contentParts []string, escapeParts []string, localizer localize.Localizer) (*PrecompiledTemplate, error) {
	if name == TemplateNameUndefined {
		return nil, errors.New("models: name is missing")
	}
	if subjectTemplate == "" {
		return nil, errors.New("models: subject template is missing")
	}
	if bodyTemplate == "" {
		return nil, errors.New("models: body template is missing")
	}

	if localizer == nil {
		return nil, errors.New("localizer is missing or null")
	}

	if contentParts == nil {
		return nil, errors.New("contentParts is missing or null")
	}

	if escapeParts == nil {
		return nil, errors.New("escapeParts is missing or null")
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
		localizer:          localizer,
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
func (p *PrecompiledTemplate) Execute(content interface{}, lang string) (string, string, error) {

	var bodyBuffer bytes.Buffer
	var subject string
	var err error
	p.fillAndLocalize(lang, content.(map[string]interface{}))

	if subject, err = p.fillAndLocalizeSubject(lang, content.(map[string]interface{})); err != nil {
		return "", "", fmt.Errorf("models: failure to generate subject %s", strconv.Quote(p.name.String()))
	}

	if err := p.precompiledBody.Execute(&bodyBuffer, content); err != nil {
		return "", "", fmt.Errorf("models: failure to execute body template %s with content", strconv.Quote(p.name.String()))
	}

	return subject, bodyBuffer.String(), nil
}

// fillAndLocalize fills the template content parts based on language bundle and locale
// A template content/body is made of HTML tags and content that can be localized
// Each template references its parts that can be filled in a collection called ContentParts
func (p *PrecompiledTemplate) fillAndLocalize(locale string, content map[string]interface{}) {
	contextParts := p.fillEscapedParts(content)
	// Get content parts from the template
	for _, v := range p.ContentParts() {
		// Each part is translated in the requested locale and added to the Content collection
		contentItem, _ := p.localizer.Localize(v, locale, contextParts)
		content[v] = contentItem
	}
}

func (p *PrecompiledTemplate) fillAndLocalizeSubject(locale string, content map[string]interface{}) (string, error) {
	contextParts := p.fillEscapedParts(content)
	// Get content parts from the template
	return p.localizer.Localize(p.Subject(), locale, contextParts)
}

// fillEscapedParts dynamically fills the escape parts with content
func (p *PrecompiledTemplate) fillEscapedParts(content map[string]interface{}) map[string]interface{} {

	// Escaped parts are replaced with content value
	var escape = make(map[string]interface{})
	if p.EscapeParts() != nil {
		for _, v := range p.EscapeParts() {
			escape[v] = content[v]
		}
	}

	return escape
}
