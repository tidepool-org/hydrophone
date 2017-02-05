package templates

import "./../models"

func NewSignupCustodialTemplate() (models.Template, error) {
	return models.NewPrecompiledTemplate(models.TemplateNameSignupCustodial, _SignupSubjectTemplate, _SignupBodyTemplate)
}
