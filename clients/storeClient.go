package clients

import (
	"context"

	"github.com/tidepool-org/hydrophone/models"
)

// FilterOpts allows further filtering options of Confirmation retrieval for more specific use cases.
// The zero value is fine for the "default" case of filtering on the logical AND of non empty fields.
type FilterOpts struct {
	AllowEmptyUserID bool // If true, then specifically query for an empty string userId instead of not including in the query.
}

type StoreClient interface {
	Ping(ctx context.Context) error
	UpsertConfirmation(ctx context.Context, confirmation *models.Confirmation) error
	FindConfirmations(ctx context.Context, confirmation *models.Confirmation, statuses ...models.Status) (results []*models.Confirmation, err error)
	FindConfirmationsWithOpts(ctx context.Context, confirmation *models.Confirmation, opts FilterOpts, statuses ...models.Status) (results []*models.Confirmation, err error)
	FindConfirmation(ctx context.Context, confirmation *models.Confirmation) (result *models.Confirmation, err error)
	RemoveConfirmation(ctx context.Context, confirmation *models.Confirmation) error
	RemoveConfirmationsForUser(ctx context.Context, userId string) error
}
