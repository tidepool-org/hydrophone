package clients

import (
	"./../models"
)

type StoreClient interface {
	Close()
	Ping() error
	UpsertConfirmation(confirmation *models.Confirmation) error
	ConfirmationsToEmail(userEmail string, statuses ...models.Status) (results []*models.Confirmation, err error)
	ConfirmationsToUser(userId string, statuses ...models.Status) (results []*models.Confirmation, err error)
	ConfirmationsFromUser(creatorId string, statuses ...models.Status) (results []*models.Confirmation, err error)
	FindConfirmation(confirmation *models.Confirmation) (result *models.Confirmation, err error)
	FindConfirmationByKey(key string) (result *models.Confirmation, err error)
	RemoveConfirmation(confirmation *models.Confirmation) error
}
