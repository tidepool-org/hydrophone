package models

import (
	"strings"
	"testing"
)

const EMAIL_ADDRESS = "user@test.org"

var (
	//config templates
	cfg = &EmailTemplate{
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

func TestEmail(t *testing.T) {

	email, err := NewEmailNotification(CONFIRMATION, cfg, EMAIL_ADDRESS)

	if err != nil {
		t.Fatalf("unexpected error ", err)
	}

	if email.ToUser != EMAIL_ADDRESS {
		t.Fatal("the user being emailed should be set")
	}

	if email.Content == "" {
		t.Fatal("the content of the email should be set")
	}

	if strings.Contains(email.Content, EMAIL_ADDRESS) == false {
		t.Fatal("the name should be set")
	}

	if strings.Contains(email.Content, email.Key) == false {
		t.Fatal("the key should be used")
	}

	if strings.Contains(email.Key, CONFIRMATION) == false {
		t.Fatal("the key should include the type")
	}

	if email.Key == "" {
		t.Fatal("the content of the email should be set")
	}

	if email.Created.IsZero() {
		t.Fatal("the date the email was created should be set")
	}
}

func TestEmailSend(t *testing.T) {

	email, _ := NewEmailNotification(CARETEAM_INVITE, cfg, EMAIL_ADDRESS)

	if email.Sent.IsZero() == false {
		t.Fatal("the time sent should not be set")
	}

	email.Send()

	if email.Sent.IsZero() {
		t.Fatal("the time sent should have been set")
	}

}
