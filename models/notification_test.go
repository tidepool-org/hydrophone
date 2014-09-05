package models

import (
	"strings"
	"testing"
)

var (
	//users
	EMAIL_USER = &User{Id: "123-99-100", Name: "To User", Emails: []string{"to@new.bar"}}
	FROM_USER  = &User{Id: "456-99-100", Name: "From user", Emails: []string{"from@new.bar"}}
	//config templates
	cfg = &EmailTemplate{
		PasswordReset: `
{{define "reset_test"}}
## Test Template
Hi {{ .ToUser.Name }}
{{ .Key }}
{{end}}
{{template "reset_test" .}}
`,
		CareteamInvite: `
{{define "invite_test"}}
## Test Template
{{ .ToUser.Name }}
{{ .Key }}
{{ .FromUser.Name }}
{{end}}
{{template "invite_test" .}}
`, Confirmation: `
{{define "confirm_test"}}
## Test Template
{{ .User.Name }}
{{ .Key }}
{{end}}
{{template "confirm_test" .}}
`}
)

func TestEmail(t *testing.T) {

	email, err := NewEmail(PW_RESET, cfg, EMAIL_USER)

	if err != nil {
		t.Fatalf("unexpected error ", err)
	}

	if email.ToUser != EMAIL_USER {
		t.Fatal("the user being emailed should be set")
	}

	if email.Content == "" {
		t.Fatal("the content of the email should be set")
	}

	if strings.Contains(email.Content, EMAIL_USER.Name) == false {
		t.Fatal("the name should be set")
	}

	if strings.Contains(email.Content, email.Key) == false {
		t.Fatal("the key should be used")
	}

	if strings.Contains(email.Key, PW_RESET) == false {
		t.Fatal("the key should include the type")
	}

	if email.Key == "" {
		t.Fatal("the content of the email should be set")
	}

	if email.FromUser != nil {
		t.Fatal("the FromUser should be empty")
	}

	if email.Created.IsZero() {
		t.Fatal("the date the email was created should be set")
	}
}

func TestEmailSend(t *testing.T) {

	email, _ := NewEmail(PW_RESET, cfg, EMAIL_USER)

	if email.Sent.IsZero() == false {
		t.Fatal("the time sent should not be set")
	}

	email.Send()

	if email.Sent.IsZero() {
		t.Fatal("the time sent should have been set")
	}

}
