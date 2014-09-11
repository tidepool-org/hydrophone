package models

import (
	"bytes"
	"html/template"
	"log"
)

type (
	TemplateConfig struct {
		PasswordReset  string `json:"passwordReset"`
		CareteamInvite string `json:"careteamInvite"`
		Confirmation   string `json:"confirmation"`
	}

	Template struct {
		compiled *template.Template
		parsed   string
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

	switch {
	case templateType == CARETEAM_INVITE:
		compiled = template.Must(template.New(string(CARETEAM_INVITE)).Parse(cfg.CareteamInvite))
		break
	case templateType == CONFIRMATION:
		compiled = template.Must(template.New(string(CONFIRMATION)).Parse(cfg.Confirmation))
		break
	case templateType == PW_RESET:
		compiled = template.Must(template.New(string(PW_RESET)).Parse(cfg.PasswordReset))
		break
	default:
		log.Println("Unknown type ", templateType)
		compiled = nil
		break
	}

	t.compiled = compiled
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

	t.parsed = buffer.String()
}
