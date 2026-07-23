package templates

import "github.com/tidepool-org/hydrophone/models"

const _SignupCustodialNewClinicExperienceReminderSubjectTemplate string = `Reminder: Accept Your Invitation from {{ .ClinicName }} to Share Your Diabetes Data`

func NewSignupCustodialNewClinicExperienceReminderTemplate() (models.Template, error) {
	return models.NewPrecompiledTemplate(models.TemplateNameSignupCustodialNewClinicExperienceReminder, _SignupCustodialNewClinicExperienceReminderSubjectTemplate, _SignupCustodialNewClinicExperienceBodyTemplate)
}
