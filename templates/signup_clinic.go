package templates

import "./../models"

func NewSignupClinicTemplate() (models.Template, error) {
	return models.NewPrecompiledTemplate(models.TemplateNameSignupClinic, _SignupSubjectTemplate, _SignupBodyTemplate)
}
