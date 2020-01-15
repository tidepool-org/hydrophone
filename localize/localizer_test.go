package localize

import (
	"testing"
)

const TestCreatorName = "Chuck Norris"

const (
	expectedLocalizedContent = "This is a test content created by " + TestCreatorName + "."
	expectedSubject          = "This email is here for testing purposes." // The subject we expect to find after compilation and localization
	locale                   = "en"
)

func Test_CreateLocalizerBundle(t *testing.T) {

	// Create a Bundle to use for the lifetime of your application
	var localizer *I18nLocalizer
	var err error

	localizer, err = NewI18nLocalizer("shouldFail")
	if err == nil || localizer != nil {
		t.Fatalf("Creation of the localizer should have failed when called with a wrong path")
	}

	localizer, err = NewI18nLocalizer(".")
	if err == nil || localizer != nil {
		t.Fatalf("Creation of the localizer should have failed when called with a wrong path")
	}

	localizer, err = NewI18nLocalizer("./test_fixture/")
	if err != nil || localizer == nil {
		t.Fatalf("Failed to create bundle: %s", err.Error())
	}

}

func Test_GetLocalizedPart(t *testing.T) {

	// Create a Bundle to use for the lifetime of your application
	localizer, err := NewI18nLocalizer("./test_fixture/")

	if localizer == nil {
		t.Fatalf("Failed to create bundle: %s", err.Error())
	}

	// For each content that needs to be filled, add localized content to a temp variable "content"
	content := make(map[string]interface{})
	content["TestCreatorName"] = TestCreatorName
	localizedContent, _ := localizer.Localize("TestContentInjection", locale, content)

	if localizedContent != expectedLocalizedContent {
		t.Fatalf("Wrong localized content, expecting %s but found %s", expectedLocalizedContent, localizedContent)
	}

	localizedContent, _ = localizer.Localize("TestTemplateSubject", locale, nil)

	if localizedContent != expectedSubject {
		t.Fatalf("Wrong localized content, expecting %s but found %s", expectedSubject, localizedContent)
	}

	localizedContent, err = localizer.Localize("wrongKey", locale, nil)

	if err == nil {
		t.Fatalf("Localization should have failed when called with a wrong key")
	}
}
