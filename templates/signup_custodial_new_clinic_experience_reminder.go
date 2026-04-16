package templates

import "github.com/tidepool-org/hydrophone/models"

const _SignupCustodialNewClinicExperienceReminderSubjectTemplate string = `{{ .ClinicName }} Follow Up - Reminder to Claim your Tidepool Account`

func NewSignupCustodialNewClinicExperienceReminderTemplate() (models.Template, error) {
	return models.NewPrecompiledTemplate(models.TemplateNameSignupCustodialNewClinicExperienceReminder, _SignupCustodialNewClinicExperienceReminderSubjectTemplate, _SignupCustodialNewClinicExperienceBodyTemplate)
}
