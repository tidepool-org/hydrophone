package clients

import (
	"./../models"
)

type StoreClient interface {
	Close()
	Ping() error
	UpsertConfirmation(confirmation *models.Confirmation) error
	FindConfirmation(confirmation *models.Confirmation) (result *models.Confirmation, err error)
	RemoveConfirmation(confirmation *models.Confirmation) error
}
