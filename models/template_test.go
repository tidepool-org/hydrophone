package models

import (
	"strings"
	"testing"
)

type (
	Data struct {
		ToUser string
		Key    string
	}
)

var (
	content = Data{ToUser: "Test User", Key: "blah.blah.blah"}
	//config templates
	cfg = &TemplateConfig{
		PasswordReset: `
{{define "reset_test"}}
## Test Template
Hi {{ .ToUser }}
{{ .Key }}
{{end}}
{{template "reset_test" .}}
`,
		CareteamInvite: `
{{define "invite_test"}}
## Test Template
{{ .ToUser }}
{{ .Key }}
{{end}}
{{template "invite_test" .}}
`, Confirmation: `
{{define "confirm_test"}}
## Test Template
{{ .ToUser }}
{{ .Key }}
{{end}}
{{template "confirm_test" .}}
`}
)

func TestLoad(t *testing.T) {

	tmpl := NewTemplate()

	tmpl.Load(PW_RESET, cfg)

	if tmpl.compiled == nil {
		t.Fatal("a template should have been created")
	}

	if tmpl.compiled.Name() != string(PW_RESET) {
		t.Fatalf("the name is [%s] but should be [%s]", tmpl.compiled.Name(), string(PW_RESET))
	}

	if tmpl.parsed != "" {
		t.Fatalf("Parsed content should be empty but is [%s]", tmpl.parsed)
	}
}

func TestParse_WhenNoLoadedTemplate(t *testing.T) {

	tmpl := NewTemplate()

	tmpl.Parse(content)

	if tmpl.parsed != "" {
		t.Fatal("parsed content should be empty as template is not set")
	}
}

func TestParse(t *testing.T) {

	tmpl := NewTemplate()

	tmpl.Load(CONFIRMATION, cfg)

	tmpl.Parse(content)

	if tmpl.parsed == "" {
		t.Fatal("The parased content should be set")
	}

	if strings.Contains(tmpl.parsed, content.ToUser) == false {
		t.Fatal("the name should be set")
	}

	if strings.Contains(tmpl.parsed, content.Key) == false {
		t.Fatal("the key should be set")
	}

}
