package clients

import "github.com/tidepool-org/hydrophone/models"

type StoreClient interface {
	Ping() error
	UpsertConfirmation(confirmation *models.Confirmation) error
	FindConfirmations(confirmation *models.Confirmation, statuses ...models.Status) (results []*models.Confirmation, err error)
	FindConfirmation(confirmation *models.Confirmation) (result *models.Confirmation, err error)
	RemoveConfirmation(confirmation *models.Confirmation) error
}
