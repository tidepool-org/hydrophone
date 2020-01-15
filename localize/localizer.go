// localize contains all we need to localize content (i.e. web pages)
// It is based on nicksnyder i18n library
package localize

import (
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"path/filepath"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	yaml "gopkg.in/yaml.v2"
)

type Localizer interface {
	Localize(key string, locale string, data map[string]interface{}) (string, error)
}

type I18nLocalizer struct {
	bundle *i18n.Bundle
}

// createLocalizer initializes the internationalization objects needed by the api
// Ensure at least en.yaml is present in the folder specified by TIDEPOOL_HYDROPHONE_SERVICE environment variable
func NewI18nLocalizer(localesPath string) (*I18nLocalizer, error) {

	// Get all the language files that exist
	langFiles, err := getAllLocalizationFiles(localesPath)

	if err != nil {
		return nil, fmt.Errorf("Error initializing localization, %s", err)
	}

	// Bundle stores a set of messages
	bundle := i18n.NewBundle(language.English)

	// Enable bundle to understand yaml
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

	var translations []byte
	for _, file := range langFiles {

		// Read our language yaml file
		translations, err = ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("Unable to read translation file %s", file)
		}

		// It parses the bytes in buffer to add translations to the bundle
		bundle.MustParseMessageFileBytes(translations, file)
	}

	log.Printf("Localizer bundle created with default language: english")
	return &I18nLocalizer{bundle}, nil
}

// getLocalizedContentPart returns translated content part based on key and locale
func (l *I18nLocalizer) Localize(key string, locale string, data map[string]interface{}) (string, error) {
	localizer := i18n.NewLocalizer(l.bundle, locale)
	msg, err := localizer.Localize(
		&i18n.LocalizeConfig{
			MessageID:    key,
			TemplateData: data,
		},
	)
	if msg == "" {
		msg = "<< Cannot find translation for item " + key + " >>"
	}
	return msg, err
}

// getAllLocalizationFiles returns all the filenames within the folder specified by the TIDEPOOL_HYDROPHONE_SERVICE environment variable
// Add yaml file to this folder to get a language added
// At least en.yaml should be present
func getAllLocalizationFiles(dir string) ([]string, error) {

	log.Printf("getting localization files from %s", dir)
	var retFiles []string
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Printf("Can't read directory %s", dir)
		return nil, err
	}

	for _, file := range files {
		filePath, _ := filepath.Abs(path.Join(dir, file.Name()))
		if !file.IsDir() && file.Name() != "test.en.yaml" && (filepath.Ext(filePath) == ".yaml" || filepath.Ext(filePath) == ".yml") {
			log.Printf("Found localization file %s", filePath)
			retFiles = append(retFiles, filePath)
		}
	}
	if len(retFiles) < 1 {
		return nil, fmt.Errorf("No locale files (yml or yaml extension) found in %s", dir)
	}
	return retFiles, nil
}
