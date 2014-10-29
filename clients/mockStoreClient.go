package clients

import (
	"errors"
	"time"

	"./../models"
)

type MockStoreClient struct {
	doBad      bool
	returnNone bool
	now        time.Time
}

func NewMockStoreClient(returnNone, doBad bool) *MockStoreClient {
	return &MockStoreClient{doBad: doBad, returnNone: returnNone, now: time.Now()}
}

func (d *MockStoreClient) Close() {}

func (d *MockStoreClient) Ping() error {
	if d.doBad {
		return errors.New("Session failure")
	}
	return nil
}

func dateDaysAgo(date time.Time, daysAgo int) time.Time {

	yr := date.Year()
	mnt := date.Month()
	days := date.Day()

	if days > daysAgo {
		days = days - daysAgo
	} else if mnt > 1 {
		//fallback: not enough days
		mnt = mnt - 1
		days = 30 - daysAgo
	} else {
		//fallback: start of year, go to end of previous yr
		yr = yr - 1
		mnt = 12
		days = 30 - daysAgo
	}

	return time.Date(yr, mnt, days, 0, 0, 0, 0, time.UTC)

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

	notification.Created = dateDaysAgo(d.now, 3) // created three days ago
	return notification, nil
}

func (d *MockStoreClient) FindConfirmations(confirmation *models.Confirmation, statuses ...models.Status) (results []*models.Confirmation, err error) {
	if d.doBad {
		return nil, errors.New("FindConfirmation failure")
	}
	if d.returnNone {
		return nil, nil
	}

	confirmation.Created = dateDaysAgo(d.now, 3) // created three days ago
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
