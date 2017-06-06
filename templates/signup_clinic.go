package templates

import "github.com/tidepool-org/hydrophone/models"

func NewSignupClinicTemplate() (models.Template, error) {
	return models.NewPrecompiledTemplate(models.TemplateNameSignupClinic, _SignupSubjectTemplate, _SignupBodyTemplate)
}
