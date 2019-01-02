package templates

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/tidepool-org/hydrophone/models"
)

type TemplateMeta struct {
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	TemplateFilename   string   `json:"templateFilename"`
	ContentParts       []string `json:"contentParts"`
	Subject            string   `json:"subject"`
	EscapeContentParts []string `json:"escapeContentParts"`
}

func New(templatesPath string) (models.Templates, error) {
	templates := models.Templates{}

	if template, err := NewTemplate(templatesPath, models.TemplateNameCareteamInvite); err != nil {
		return nil, fmt.Errorf("templates: failure to create careteam invite template: %s", err)
	} else {
		templates[template.Name()] = template
	}

	if template, err := NewTemplate(templatesPath, models.TemplateNameNoAccount); err != nil {
		return nil, fmt.Errorf("templates: failure to create no account template: %s", err)
	} else {
		templates[template.Name()] = template
	}

	if template, err := NewTemplate(templatesPath, models.TemplateNamePasswordReset); err != nil {
		return nil, fmt.Errorf("templates: failure to create password reset template: %s", err)
	} else {
		templates[template.Name()] = template
	}

	if template, err := NewTemplate(templatesPath, models.TemplateNameSignup); err != nil {
		return nil, fmt.Errorf("templates: failure to create signup template: %s", err)
	} else {
		templates[template.Name()] = template
	}

	if template, err := NewTemplate(templatesPath, models.TemplateNameSignupClinic); err != nil {
		return nil, fmt.Errorf("templates: failure to create signup clinic template: %s", err)
	} else {
		templates[template.Name()] = template
	}

	if template, err := NewTemplate(templatesPath, models.TemplateNameSignupCustodial); err != nil {
		return nil, fmt.Errorf("templates: failure to create signup custodial template: %s", err)
	} else {
		templates[template.Name()] = template
	}

	if template, err := NewTemplate(templatesPath, models.TemplateNameSignupCustodialClinic); err != nil {
		return nil, fmt.Errorf("templates: failure to create signup custodial clinic template: %s", err)
	} else {
		templates[template.Name()] = template
	}

	return templates, nil
}

//NewTemplate returns the requested template
//templateName is the name of the template to be returned
func NewTemplate(templatesPath string, templateName models.TemplateName) (models.Template, error) {
	// Get template Metadata
	var templateMeta = getTemplateMeta(templatesPath + "/meta/" + string(templateName) + ".json")
	var templateFileName = templatesPath + "/html/" + templateMeta.TemplateFilename

	return models.NewPrecompiledTemplate(templateName, templateMeta.Subject, getBodySkeleton(templateFileName), templateMeta.ContentParts, templateMeta.EscapeContentParts)
}

// getTemplateMeta returns the template metadata
// Metadata are information that relate to a template (e.g. name, templateFilename...)
// Inputs:
// metaFileName = name of the file with no path and no json extension, assuming the file is located in path specified in TIDEPOOL_HYDROPHONE_SERVICE environment variable
func getTemplateMeta(metaFileName string) TemplateMeta {
	log.Printf("getting template meta from %s", metaFileName)

	// Open the jsonFile
	jsonFile, err := os.Open(metaFileName)
	if err != nil {
		fmt.Println(err)
	}

	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	// read the opened xmlFile as a byte array.
	byteValue, _ := ioutil.ReadAll(jsonFile)

	var meta TemplateMeta

	// we unmarshal our byteArray which contains our
	// jsonFile's content into 'users' which we defined above
	json.Unmarshal(byteValue, &meta)

	return meta
}

// getBodySkeleton returns the email body skeleton (without content) from the file which name is in input parameter
func getBodySkeleton(fileName string) string {

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Printf("templates - failure to get template body: %s", err)
	}
	log.Printf("getting template body from %s", fileName)
	return string(data)
}
