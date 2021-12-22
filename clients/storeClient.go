package clients

import (
	"context"
	"time"

	goComMgo "github.com/mdblp/go-common/clients/mongo"
	"github.com/mdblp/hydrophone/models"
)

type StoreClient interface {
	goComMgo.Storage
	UpsertConfirmation(ctx context.Context, confirmation *models.Confirmation) error
	FindConfirmations(ctx context.Context, confirmation *models.Confirmation, statuses []models.Status, types []models.Type) (results []*models.Confirmation, err error)
	FindConfirmation(ctx context.Context, confirmation *models.Confirmation) (result *models.Confirmation, err error)
	RemoveConfirmation(ctx context.Context, confirmation *models.Confirmation) error
	CountLatestConfirmations(ctx context.Context, confirmation models.Confirmation, createdSince time.Time) (int64, error)
}
