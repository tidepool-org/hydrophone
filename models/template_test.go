package models

import "testing"

type (
	Data struct {
		Username string
		Key      string
	}
)

var (
	content = Data{Username: "Test User", Key: "123.blah.456.blah"}

	name TemplateName = "test"

	subjectSuccessTemplate = `Username is '{{ .Username }}'`
	subjectFailureTemplate = `{{define "subjectFailure"}}`

	bodySuccessTemplate = `Key is '{{ .Key }}'`
	bodyFailureTemplate = `{{define "bodyFailure"}}`
)

func Test_NewPrecompiledTemplate_NameMissing(t *testing.T) {
	expectedError := "models: name is missing"
	tmpl, err := NewPrecompiledTemplate("", subjectSuccessTemplate, bodySuccessTemplate)
	if err == nil || err.Error() != expectedError {
		t.Fatalf(`Error is "%s", but should be "%s"`, err, expectedError)
	}
	if tmpl != nil {
		t.Fatal("Template should be nil")
	}
}

func Test_NewPrecompiledTemplate_SubjectTemplateMissing(t *testing.T) {
	expectedError := "models: subject template is missing"
	tmpl, err := NewPrecompiledTemplate(name, "", bodySuccessTemplate)
	if err == nil || err.Error() != expectedError {
		t.Fatalf(`Error is "%s", but should be "%s"`, err, expectedError)
	}
	if tmpl != nil {
		t.Fatal("Template should be nil")
	}
}

func Test_NewPrecompiledTemplate_BodyTemplateMissing(t *testing.T) {
	expectedError := "models: body template is missing"
	tmpl, err := NewPrecompiledTemplate(name, subjectSuccessTemplate, "")
	if err == nil || err.Error() != expectedError {
		t.Fatalf(`Error is "%s", but should be "%s"`, err, expectedError)
	}
	if tmpl != nil {
		t.Fatal("Template should be nil")
	}
}

func Test_NewPrecompiledTemplate_SubjectTemplateNotPrecompiled(t *testing.T) {
	expectedError := "models: failure to precompile subject template: template: test:1: unexpected EOF"
	tmpl, err := NewPrecompiledTemplate(name, subjectFailureTemplate, bodySuccessTemplate)
	if err == nil || err.Error() != expectedError {
		t.Fatalf(`Error is "%s", but should be "%s"`, err, expectedError)
	}
	if tmpl != nil {
		t.Fatal("Template should be nil")
	}
}

func Test_NewPrecompiledTemplate_BodyTemplateNotPrecompiled(t *testing.T) {
	expectedError := "models: failure to precompile body template: template: test:1: unexpected EOF"
	tmpl, err := NewPrecompiledTemplate(name, subjectSuccessTemplate, bodyFailureTemplate)
	if err == nil || err.Error() != expectedError {
		t.Fatalf(`Error is "%s", but should be "%s"`, err, expectedError)
	}
	if tmpl != nil {
		t.Fatal("Template should be nil")
	}
}

func Test_NewPrecompiledTemplate_Success(t *testing.T) {
	tmpl, err := NewPrecompiledTemplate(name, subjectSuccessTemplate, bodySuccessTemplate)
	if err != nil {
		t.Fatalf(`Error is "%s", but should be nil`, err)
	}
	if tmpl == nil {
		t.Fatal("Template should be not nil")
	}
}

func Test_NewPrecompiledTemplate_Name(t *testing.T) {
	tmpl, _ := NewPrecompiledTemplate(name, subjectSuccessTemplate, bodySuccessTemplate)
	if tmpl.Name() != name {
		t.Fatalf(`Name is "%s", but should be "%s"`, tmpl.Name(), name)
	}
}

func Test_NewPrecompiledTemplate_ExecuteSuccess(t *testing.T) {
	expectedSubject := `Username is 'Test User'`
	expectedBody := `Key is '123.blah.456.blah'`
	tmpl, _ := NewPrecompiledTemplate(name, subjectSuccessTemplate, bodySuccessTemplate)
	subject, body, err := tmpl.Execute(content)
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
