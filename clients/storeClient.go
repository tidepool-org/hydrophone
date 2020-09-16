package clients

import (
	goComMgo "github.com/tidepool-org/go-common/clients/mongo"
	"github.com/tidepool-org/hydrophone/models"
)

type StoreClient interface {
	goComMgo.Storage
	UpsertConfirmation(confirmation *models.Confirmation) error
	FindConfirmations(confirmation *models.Confirmation, statuses ...models.Status) (results []*models.Confirmation, err error)
	FindConfirmation(confirmation *models.Confirmation) (result *models.Confirmation, err error)
	RemoveConfirmation(confirmation *models.Confirmation) error
}
