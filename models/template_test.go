package models

import (
	"testing"

	"github.com/mdblp/hydrophone/localize"
)

type (
	Data struct {
		Username string
		Key      string
	}
)

const TestCreatorName = "Chuck Norris"

var (
	name TemplateName = "test"

	subjectSuccessTemplate = `subject`
	subjectFailureTemplate = `{{define "subjectFailure"}}`
	translations           = map[string]string{
		"subject": "Username is 'Test User'",
		"Key":     "123.blah.456.blah",
	}

	bodySuccessTemplate = `Key is '{{ .Key }}'`
	bodyFailureTemplate = `{{define "bodyFailure"}}`
	localizer           = localize.NewMockLocalizer(translations)

	contentPart = []string{"Key"}
	espacePart  = []string{"Username"}
)

const (
	expectedLocalizedContent = "This is a test content created by " + TestCreatorName + "."
	expectedSubject          = "This email is here for testing purposes." // The subject we expect to find after compilation and localization
	locale                   = "en"
)

func assertFailure(t *testing.T, template *PrecompiledTemplate, err error, expectedError string) {
	if err == nil || err.Error() != expectedError {
		t.Fatalf(`Error is "%s", but should be "%s"`, err, expectedError)
	}
	if template != nil {
		t.Fatal("Template should be nil")
	}
}

func Test_NewPrecompiledTemplate_NameMissing(t *testing.T) {
	expectedError := "models: name is missing"
	tmpl, err := NewPrecompiledTemplate("", subjectSuccessTemplate, bodySuccessTemplate, contentPart, espacePart, localizer)
	assertFailure(t, tmpl, err, expectedError)
}

func Test_NewPrecompiledTemplate_SubjectTemplateMissing(t *testing.T) {
	expectedError := "models: subject template is missing"
	tmpl, err := NewPrecompiledTemplate(name, "", bodySuccessTemplate, contentPart, espacePart, localizer)
	assertFailure(t, tmpl, err, expectedError)
}

func Test_NewPrecompiledTemplate_BodyTemplateMissing(t *testing.T) {
	expectedError := "models: body template is missing"
	tmpl, err := NewPrecompiledTemplate(name, subjectSuccessTemplate, "", contentPart, espacePart, localizer)
	assertFailure(t, tmpl, err, expectedError)
}

func Test_NewPrecompiledTemplate_LocalizerMissing(t *testing.T) {
	expectedError := "localizer is missing or null"
	tmpl, err := NewPrecompiledTemplate(name, subjectSuccessTemplate, bodySuccessTemplate, contentPart, espacePart, nil)
	assertFailure(t, tmpl, err, expectedError)
}

func Test_NewPrecompiledTemplate_ContentPartsMissing(t *testing.T) {
	expectedError := "contentParts is missing or null"
	tmpl, err := NewPrecompiledTemplate(name, subjectSuccessTemplate, bodySuccessTemplate, nil, espacePart, localizer)
	assertFailure(t, tmpl, err, expectedError)
}

func Test_NewPrecompiledTemplate_TokensMissing(t *testing.T) {
	expectedError := "escapeParts is missing or null"
	tmpl, err := NewPrecompiledTemplate(name, subjectSuccessTemplate, bodySuccessTemplate, contentPart, nil, localizer)
	assertFailure(t, tmpl, err, expectedError)
}
func Test_NewPrecompiledTemplate_SubjectTemplateNotPrecompiled(t *testing.T) {
	expectedError := "models: failure to precompile subject template: template: test:1: unexpected EOF"
	tmpl, err := NewPrecompiledTemplate(name, subjectFailureTemplate, bodySuccessTemplate, contentPart, espacePart, localizer)
	assertFailure(t, tmpl, err, expectedError)
}

func Test_NewPrecompiledTemplate_BodyTemplateNotPrecompiled(t *testing.T) {
	expectedError := "models: failure to precompile body template: template: test:1: unexpected EOF"
	tmpl, err := NewPrecompiledTemplate(name, subjectSuccessTemplate, bodyFailureTemplate, contentPart, espacePart, localizer)
	assertFailure(t, tmpl, err, expectedError)
}

func Test_NewPrecompiledTemplate_Success(t *testing.T) {
	tmpl, err := NewPrecompiledTemplate(name, subjectSuccessTemplate, bodySuccessTemplate, contentPart, espacePart, localizer)
	if err != nil {
		t.Fatalf(`Error is "%s", but should be nil`, err)
	}
	if tmpl == nil {
		t.Fatal("Template should be not nil")
	}
}

func Test_NewPrecompiledTemplate_Name(t *testing.T) {
	tmpl, _ := NewPrecompiledTemplate(name, subjectSuccessTemplate, bodySuccessTemplate, contentPart, espacePart, localizer)
	if tmpl.Name() != name {
		t.Fatalf(`Name is "%s", but should be "%s"`, tmpl.Name(), name)
	}
}

func Test_NewPrecompiledTemplate_ExecuteSuccess(t *testing.T) {
	content := make(map[string]interface{})
	content["Username"] = "Test User"
	expectedSubject := `Username is 'Test User'`
	expectedBody := `Key is '123.blah.456.blah'`
	tmpl, _ := NewPrecompiledTemplate(name, subjectSuccessTemplate, bodySuccessTemplate, contentPart, espacePart, localizer)
	subject, body, err := tmpl.Execute(content, "en")
	if err != nil {
		t.Fatalf(`Error is "%s", but should be nil`, err)
	}
	if subject != expectedSubject {
		t.Fatalf(`Subject is "%s", but should be "%s"`, subject, expectedSubject)
	}
	if body != expectedBody {
		t.Fatalf(`Body is "%s", but should be "%s"`, body, expectedBody)
	}
}

func Test_NewPrecompiledTemplate_ExecuteFailure(t *testing.T) {
	content := make(map[string]interface{})
	content["Username2"] = "Test User"
	// Should fail if the subject cannot be localized
	tmpl, _ := NewPrecompiledTemplate(name, "subject2", bodySuccessTemplate, contentPart, espacePart, localizer)
	_, _, err := tmpl.Execute(content, "en")
	if err == nil {
		t.Fatalf(`Error should be "%s", but is nil`, "models: failure to generate subject \"test\"")
	}
}
