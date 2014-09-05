package clients

import (
	"./../models"
)

type StoreClient interface {
	Close()
	Ping() error
	UpsertNotification(notification *models.Notification) error
	FindNotification(notification *models.Notification) (result *models.Notification, err error)
	RemoveNotification(notification *models.Notification) error
}
