package templates

import "github.com/tidepool-org/hydrophone/models"

const _SignupCustodialNewClinicExperienceReminderSubjectTemplate string = `Reminder: Accept Your Tidepool Invitation from {{ .ClinicName }}`

func NewSignupCustodialNewClinicExperienceReminderTemplate() (models.Template, error) {
	return models.NewPrecompiledTemplate(models.TemplateNameSignupCustodialNewClinicExperienceReminder, _SignupCustodialNewClinicExperienceReminderSubjectTemplate, _SignupCustodialNewClinicExperienceBodyTemplate)
}
