package clients

import (
	"./../models"
	"errors"
)

type MockStoreClient struct {
	doBad           bool
	returnDifferent bool
}

func NewMockStoreClient(returnDifferent, doBad bool) *MockStoreClient {
	return &MockStoreClient{doBad: doBad, returnDifferent: returnDifferent}
}

func (d MockStoreClient) Close() {}

func (d MockStoreClient) Ping() error {
	if d.doBad {
		return errors.New("Session failure")
	}
	return nil
}

func (d MockStoreClient) UpsertNotification(notification *models.Notification) error {
	if d.doBad {
		return errors.New("UpsertNotification failure")
	}
	return nil
}

func (d MockStoreClient) FindNotification(notification *models.Notification) (result *models.Notification, err error) {
	if d.doBad {
		return nil, errors.New("RemoveNotification failure")
	}
	return notification, nil
}

func (d MockStoreClient) RemoveNotification(notification *models.Notification) error {
	if d.doBad {
		return errors.New("RemoveNotification failure")
	}
	return nil
}
