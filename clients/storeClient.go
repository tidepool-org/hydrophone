package clients

import (
	"context"

	"github.com/tidepool-org/hydrophone/models"
)

type StoreClient interface {
	Ping(ctx context.Context) error
	UpsertConfirmation(ctx context.Context, confirmation *models.Confirmation) error
	FindConfirmations(ctx context.Context, confirmation *models.Confirmation, statuses ...models.Status) (results []*models.Confirmation, err error)
	FindConfirmation(ctx context.Context, confirmation *models.Confirmation) (result *models.Confirmation, err error)
	RemoveConfirmation(ctx context.Context, confirmation *models.Confirmation) error
}
