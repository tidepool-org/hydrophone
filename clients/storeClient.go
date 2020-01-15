package clients

import (
	"context"

	"github.com/tidepool-org/hydrophone/models"
)

// StoreClient - interface for data storage
type StoreClient interface {
	Ping() error
	WithContext(ctx context.Context) *MongoStoreClient
	UpsertConfirmation(confirmation *models.Confirmation) error
	FindConfirmations(confirmation *models.Confirmation, statuses ...models.Status) (results []*models.Confirmation, err error)
	FindConfirmation(confirmation *models.Confirmation) (result *models.Confirmation, err error)
	RemoveConfirmation(confirmation *models.Confirmation) error
}
