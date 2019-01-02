package templates

import (
	"testing"
)

const (
	templatesPath = "."
	templateName  = "test_template"
)

func Test_GetTemplateMeta(t *testing.T) {
	var expectedTemplateSubject = "TestTemplateSubject"
	// Get template Metadata
	var templateMeta = getTemplateMeta(templatesPath + "/meta/" + templateName + ".json")

	if templateMeta.Subject == "" {
		t.Fatal("Template Meta cannot be found")
	}

	if templateMeta.Subject != expectedTemplateSubject {
		t.Fatalf("Template Meta is found but subject \"%s\" not the one expected \"%s\"", templateMeta.Subject, expectedTemplateSubject)
	}
}

func Test_GetBodySkeleton(t *testing.T) {
	// Get template Metadata
	var templateMeta = getTemplateMeta(templatesPath + "/meta/" + templateName + ".json")
	var templateFileName = templatesPath + "/html/" + templateMeta.TemplateFilename

	var templateBody = getBodySkeleton(templateFileName)

	if templateBody == "" {
		t.Fatalf("Template Body cannot be found: %s", templateFileName)
	}
}
