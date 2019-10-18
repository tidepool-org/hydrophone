package api

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/tidepool-org/hydrophone/models"
	"golang.org/x/text/language"
	yaml "gopkg.in/yaml.v2"
)

type LangQ struct {
	Lang string
	Q    float64
}

const (
	HEADER_LANGUAGE = "Accept-Language"
)

// InitI18n initializes the internationalization objects needed by the api
// Ensure at least en.yaml is present in the folder specified by TIDEPOOL_HYDROPHONE_SERVICE environment variable
func (a *Api) InitI18n(templatesPath string) {

	// Get all the language files that exist
	langFiles, err := getAllLocalizationFiles(templatesPath)

	if err != nil {
		log.Printf("Error getting translation files, %v", err)
	}

	// Create a Bundle to use for the lifetime of your application
	locBundle, err := createLocalizerBundle(langFiles)

	if err != nil {
		log.Printf("Error initialising localization, %v", err)
	} else {
		log.Printf("Localizer bundle created with default language: english")
	}

	a.LanguageBundle = locBundle
}

// createLocalizerBundle reads language files and registers them in i18n bundle
func createLocalizerBundle(langFiles []string) (*i18n.Bundle, error) {
	// Bundle stores a set of messages
	bundle := i18n.NewBundle(language.English)

	// Enable bundle to understand yaml
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

	var translations []byte
	var err error
	for _, file := range langFiles {

		// Read our language yaml file
		translations, err = ioutil.ReadFile(file)
		if err != nil {
			fmt.Errorf("Unable to read translation file %s", file)
			return nil, err
		}

		// It parses the bytes in buffer to add translations to the bundle
		bundle.MustParseMessageFileBytes(translations, file)
	}

	return bundle, nil
}

// getLocalizedContentPart returns translated content part based on key and locale
func getLocalizedContentPart(bundle *i18n.Bundle, key string, locale string, escape map[string]interface{}) (string, error) {
	localizer := i18n.NewLocalizer(bundle, locale)
	msg, err := localizer.Localize(
		&i18n.LocalizeConfig{
			MessageID:    key,
			TemplateData: escape,
		},
	)
	if msg == "" {
		msg = "<< Cannot find translation for item " + key + " >>"
	}
	return msg, err
}

// getLocalizedSubject returns translated subject based on key and locale
func getLocalizedSubject(bundle *i18n.Bundle, key string, locale string) (string, error) {
	return getLocalizedContentPart(bundle, key, locale, nil)
}

// fillTemplate fills the template content parts based on language bundle and locale
// A template content/body is made of HTML tags and content that can be localized
// Each template references its parts that can be filled in a collection called ContentParts
func fillTemplate(template models.Template, bundle *i18n.Bundle, locale string, content map[string]interface{}) {
	// Get content parts from the template
	for _, v := range template.ContentParts() {
		// Each part is translated in the requested locale and added to the Content collection
		contentItem, _ := getLocalizedContentPart(bundle, v, locale, fillEscapedParts(template, content))
		content[v] = contentItem
	}
}

// fillEscapedParts dynamically fills the escape parts with content
func fillEscapedParts(template models.Template, content map[string]interface{}) map[string]interface{} {

	// Escaped parts are replaced with content value
	var escape = make(map[string]interface{})
	if template.EscapeParts() != nil {
		for _, v := range template.EscapeParts() {
			escape[v] = content[v]
		}
	}

	return escape
}

// getAllLocalizationFiles returns all the filenames within the folder specified by the TIDEPOOL_HYDROPHONE_SERVICE environment variable
// Add yaml file to this folder to get a language added
// At least en.yaml should be present
func getAllLocalizationFiles(templatesPath string) ([]string, error) {

	var dir = templatesPath + "/locales/"
	log.Printf("getting localization files from %s", dir)
	var retFiles []string
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Printf("Can't read directory %s", dir)
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() && file.Name() != "test.en.yaml" {
			log.Printf("Found localization file %s", dir+file.Name())
			retFiles = append(retFiles, dir+file.Name())
		}
	}
	return retFiles, nil
}

//getBrowserPreferredLanguage returns the preferred language extracted from the request browser
func getBrowserPreferredLanguage(req *http.Request) string {

	if acptlng := req.Header.Get(HEADER_LANGUAGE); acptlng == "" {
		return ""
	} else if languages := parseAcceptLanguage(acptlng); languages == nil {
		return ""
	} else {
		// if at least 1 lang is found, we return the 2 first characters of the first lang
		// this header language item, although initially made for handling language is sometimes used to handle complete locale under form language-locale (eg FR-fr)
		// hence we take only the 2 first characters
		return languages[0].Lang[0:2]
	}
}

//parseAcceptLanguage will return array of languages extracted from given Accept-Language value
//Accept-Language value is retrieved from the Request Header
func parseAcceptLanguage(acptLang string) []LangQ {
	var lqs []LangQ

	langQStrs := strings.Split(acptLang, ",")
	for _, langQStr := range langQStrs {
		trimedLangQStr := strings.Trim(langQStr, " ")

		langQ := strings.Split(trimedLangQStr, ";")
		if len(langQ) == 1 {
			lq := LangQ{langQ[0], 1}
			lqs = append(lqs, lq)
		} else {
			qp := strings.Split(langQ[1], "=")
			q, err := strconv.ParseFloat(qp[1], 64)
			if err != nil {
				panic(err)
			}
			lq := LangQ{langQ[0], q}
			lqs = append(lqs, lq)
		}
	}
	return lqs
}
