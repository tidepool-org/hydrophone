package clients

import (
	"errors"

	"./../models"
)

type MockStoreClient struct {
	doBad      bool
	returnNone bool
}

func NewMockStoreClient(returnNone, doBad bool) *MockStoreClient {
	return &MockStoreClient{doBad: doBad, returnNone: returnNone}
}

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
	return notification, nil
}

func (d *MockStoreClient) FindConfirmationByKey(key string) (result *models.Confirmation, err error) {
	if d.doBad {
		return nil, errors.New("FindConfirmationByKey failure")
	}
	if d.returnNone {
		return nil, nil
	}
	conf, _ := models.NewConfirmation(models.TypeCareteamInvite, "")
	conf.Key = key
	conf.Email = "otherToInvite@email.com"
	conf.Context = []byte(`{"view":{}, "note":{}}`)
	return conf, nil
}

func (d *MockStoreClient) ConfirmationsToUser(fromId, toId, toEmail string, statuses ...models.Status) (results []*models.Confirmation, err error) {
	if d.doBad {
		return nil, errors.New("FindConfirmation failure")
	}
	if d.returnNone {
		return nil, nil
	}

	conf, _ := models.NewConfirmation(models.TypeCareteamInvite, "")
	if fromId != "" {
		conf.CreatorId = fromId
	}
	if toId != "" {
		conf.UserId = toId
	}
	if toEmail != "" {
		conf.Email = toEmail
	}

	conf.Context = []byte(`{"view":{}, "note":{}}`)
	conf.UpdateStatus(statuses[0])

	return []*models.Confirmation{conf}, nil

}
func (d *MockStoreClient) ConfirmationsFromUser(creatorId string, statuses ...models.Status) (results []*models.Confirmation, err error) {

	if d.doBad {
		return nil, errors.New("FindConfirmation failure")
	}
	if d.returnNone {
		return nil, nil
	}

	conf, _ := models.NewConfirmation(models.TypeCareteamInvite, creatorId)
	conf.UpdateStatus(statuses[0])
	conf.Context = []byte(`{"view":{}, "note":{}}`)

	return []*models.Confirmation{conf}, nil

}

func (d *MockStoreClient) RemoveConfirmation(notification *models.Confirmation) error {
	if d.doBad {
		return errors.New("RemoveConfirmation failure")
	}
	return nil
}
