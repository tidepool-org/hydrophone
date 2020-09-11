package clients

import (
	"errors"
	"time"

	"github.com/tidepool-org/hydrophone/models"
	"go.uber.org/fx"
)

type MockStoreClient struct {
	doBad      bool
	returnNone bool
	now        time.Time
}

func NewMockStoreClient(returnNone, doBad bool) StoreClient {
	return &MockStoreClient{doBad: doBad, returnNone: returnNone, now: time.Now()}
}

//MockStoreModule is a mock store
var MockStoreModule = fx.Options(fx.Provide(func() StoreClient { return NewMockStoreClient(false, false) }))

//MockStoreEmptyModule return nothing
var MockStoreEmptyModule = fx.Options(fx.Provide(func() StoreClient { return NewMockStoreClient(true, false) }))

//MockStoreFailsModule always fails
var MockStoreFailsModule = fx.Options(fx.Provide(func() StoreClient { return NewMockStoreClient(false, true) }))

func (d *MockStoreClient) Close() {}

func (d *MockStoreClient) Ping() error {
	if d.doBad {
		return errors.New("Session failure")
	}
	return nil
}

func (d *MockStoreClient) UpsertConfirmation(notification *models.Confirmation) error {
	if d.doBad {
		return errors.New("UpsertConfirmation failure")
	}
	return nil
}

func (d *MockStoreClient) FindConfirmation(notification *models.Confirmation) (result *models.Confirmation, err error) {
	if d.doBad {
		return nil, errors.New("FindConfirmation failure")
	}
	if d.returnNone {
		return nil, nil
	}

	if notification.UserId == "" {
		notification.UserId = notification.Key
	}
	if notification.Email == "" {
		notification.Email = notification.UserId
	}

	notification.Created = time.Now().AddDate(0, 0, -3) // created three days ago
	return notification, nil
}

func (d *MockStoreClient) FindConfirmations(confirmation *models.Confirmation, statuses ...models.Status) (results []*models.Confirmation, err error) {
	if d.doBad {
		return nil, errors.New("FindConfirmation failure")
	}
	if d.returnNone {
		return nil, nil
	}

	confirmation.Created = time.Now().AddDate(0, 0, -3) // created three days ago
	confirmation.Context = []byte(`{"view":{}, "note":{}}`)
	confirmation.UpdateStatus(statuses[0])

	return []*models.Confirmation{confirmation}, nil
}

func (d *MockStoreClient) RemoveConfirmation(notification *models.Confirmation) error {
	if d.doBad {
		return errors.New("RemoveConfirmation failure")
	}
	return nil
}
