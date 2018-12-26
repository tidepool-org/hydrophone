package api

import (
	"testing"

	"github.com/tidepool-org/hydrophone/models"
	templates "github.com/tidepool-org/hydrophone/templates"
)

const TestCreatorName = "Chuck Norris"

const (
	expectedLocalizedContent = "This is a test content created by " + TestCreatorName + "."
	expectedSubject          = "This email is here for testing purposes."                                                                                                                                 // The subject we expect to find after compilation and localization
	expectedBody             = "<html><header><title>Test Template. Please keep this file in this folder.</title></header><body>This is a test content created by " + TestCreatorName + ".</body></html>" // The HTML body we expect to find after compilation and localization
	locale                   = "en"
	templatesPath            = "../templates"
	langFile                 = "../templates/locales/test.en.yaml"
)

func Test_CreateLocalizerBundle(t *testing.T) {

	// Create a Bundle to use for the lifetime of your application
	locBundle, err := createLocalizerBundle(nil)

	if locBundle == nil {
		t.Fatalf("Failed to create bundle: %s", err.Error())
	}
}

func Test_GetLocalizedPart(t *testing.T) {
	var langFiles []string
	langFiles = append(langFiles, langFile)

	// Create a Bundle to use for the lifetime of your application
	locBundle, err := createLocalizerBundle(langFiles)

	if locBundle == nil {
		t.Fatalf("Failed to create bundle: %s", err.Error())
	}

	// For each content that needs to be filled, add localized content to a temp variable "content"
	content := make(map[string]interface{})
	content["TestCreatorName"] = TestCreatorName
	localizedContent, _ := getLocalizedContentPart(locBundle, "TestContentInjection", locale, content)

	if localizedContent != expectedLocalizedContent {
		t.Fatalf("Wrong localized content, expecting %s but found %s", expectedLocalizedContent, localizedContent)
	}
}

func Test_ExecuteTemplate(t *testing.T) {

	var langFiles []string
	langFiles = append(langFiles, langFile)

	// Create a Bundle to use for the lifetime of your application
	locBundle, err := createLocalizerBundle(langFiles)
	if locBundle == nil {
		t.Fatalf("Failed to create bundle: %s", err.Error())
	}

	// Create Test Template
	temp, err := templates.NewTemplate(templatesPath, models.TemplateNameTest)

	if temp == nil {
		t.Fatalf("Failed to create test template: %s", err.Error())
	}

	// For each content that needs to be filled, add localized content to a temp variable "content"
	content := make(map[string]interface{})
	content["TestCreatorName"] = TestCreatorName
	fillTemplate(temp, locBundle, locale, content)

	// Execute the template with provided content
	_, body, err := temp.Execute(content)
	// Get localized subject of email
	subject, err := getLocalizedSubject(locBundle, temp.Subject(), locale)

	// Check subject
	if subject != expectedSubject {
		t.Fatalf("Expecting subject %s but executed %s", expectedSubject, subject)
	}

	// Check Body
	if body != expectedBody {
		t.Fatalf("Compiled body is not the one expected (see below): \n %s \n %s", body, string(expectedBody))
	}
}
