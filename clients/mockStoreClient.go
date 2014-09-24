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

func (d MockStoreClient) UpsertConfirmation(notification *models.Confirmation) error {
	if d.doBad {
		return errors.New("UpsertConfirmation failure")
	}
	return nil
}

func (d MockStoreClient) FindConfirmation(notification *models.Confirmation) (result *models.Confirmation, err error) {
	if d.doBad {
		return nil, errors.New("FindConfirmation failure")
	}
	return notification, nil
}

func (d MockStoreClient) FindConfirmations(userEmail, creatorId string, status models.Status) (results []*models.Confirmation, err error) {
	if d.doBad {
		return nil, errors.New("FindConfirmation failure")
	}

	conf, _ := models.NewConfirmation(models.TypeCareteamInvite, "")

	conf.ToEmail = userEmail

	conf.UpdateStatus(status)
	if creatorId != "" {
		conf.CreatorId = creatorId
	}

	return []*models.Confirmation{conf}, nil

}

func (d MockStoreClient) RemoveConfirmation(notification *models.Confirmation) error {
	if d.doBad {
		return errors.New("RemoveConfirmation failure")
	}
	return nil
}
