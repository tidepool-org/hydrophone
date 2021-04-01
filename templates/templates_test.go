package templates

import (
	"testing"

	"github.com/tidepool-org/hydrophone/localize"
	"github.com/tidepool-org/hydrophone/models"
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

func Test_NewTemplates(t *testing.T) {
	mockLocalizer := localize.NewMockLocalizer(map[string]string{})
	emailTemplates, err := New(".", mockLocalizer)
	if err != nil {
		t.Fatalf("template.New() failled to execute with error %s", err)
	}
	expectedTemplates := []models.TemplateName{
		models.TemplateNameCareteamInvite,
		models.TemplateNameMedicalteamInvite,
		models.TemplateNameNoAccount,
		models.TemplateNamePasswordReset,
		models.TemplateNamePatientPasswordReset,
		models.TemplateNamePatientInformation,
		models.TemplateNameSignup,
		models.TemplateNameSignupClinic,
		models.TemplateNameSignupCustodial,
		models.TemplateNameSignupCustodialClinic,
	}
	for _, v := range expectedTemplates {
		if _, ok := emailTemplates[v]; !ok {
			t.Fatalf("template.New() failled to create template %s", v)
		}
	}
}
