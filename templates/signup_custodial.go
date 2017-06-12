package templates

import "github.com/tidepool-org/hydrophone/models"

func NewSignupCustodialTemplate() (models.Template, error) {
	return models.NewPrecompiledTemplate(models.TemplateNameSignupCustodial, _SignupSubjectTemplate, _SignupBodyTemplate)
}
