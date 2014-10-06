package models

import (
	"strings"
	"testing"
)

type (
	Data struct {
		UserName string
		Key      string
	}
)

var (
	content = Data{UserName: "Test User", Key: "123.blah.456.blah"}
	//config templates
	cfg = &TemplateConfig{
		PasswordReset: `
{{define "reset_test"}}
## Test Template
Hi {{ .UserName }}
{{ .Key }}
{{end}}
{{template "reset_test" .}}
`,
		CareteamInvite: `
{{define "invite_test"}}
## Test Template
{{ .UserName }}
{{ .Key }}
{{end}}
{{template "invite_test" .}}
`, Confirmation: `
{{define "confirm_test"}}
## Test Template
{{ .UserName }}
{{ .Key }}
{{end}}
{{template "confirm_test" .}}
`}
)

func TestLoad(t *testing.T) {

	tmpl := NewTemplate()

	tmpl.Load(TypePasswordReset, cfg)

	if tmpl.compiled == nil {
		t.Fatal("a template should have been created")
	}

	if tmpl.compiled.Name() != string(TypePasswordReset) {
		t.Fatalf("the name is [%s] but should be [%s]", tmpl.compiled.Name(), string(TypePasswordReset))
	}

	if tmpl.GenerateContent != "" {
		t.Fatalf("Parsed content should be empty but is [%s]", tmpl.GenerateContent)
	}
}

func TestParse_WhenNoLoadedTemplate(t *testing.T) {

	tmpl := NewTemplate()

	tmpl.Parse(content)

	if tmpl.GenerateContent != "" {
		t.Fatal("parsed content should be empty as template is not set")
	}
}

func TestParse(t *testing.T) {

	tmpl := NewTemplate()

	tmpl.Load(TypeConfirmation, cfg)

	tmpl.Parse(content)

	if tmpl.GenerateContent == "" {
		t.Fatal("The parased content should be set")
	}

	if strings.Contains(tmpl.GenerateContent, content.UserName) == false {
		t.Fatal("the name should be set")
	}

	if strings.Contains(tmpl.GenerateContent, content.Key) == false {
		t.Fatal("the key should be set")
	}

}
