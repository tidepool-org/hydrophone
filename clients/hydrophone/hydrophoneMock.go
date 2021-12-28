package hydrophone

import (
	"github.com/mdblp/hydrophone/models"
	"github.com/stretchr/testify/mock"
)

type HydrophoneMockClient struct {
	mock.Mock
	MockedError    error
	MockedConfirms []models.Confirmation
}

func NewMock() *HydrophoneMockClient {
	return &HydrophoneMockClient{
		MockedError:    nil,
		MockedConfirms: []models.Confirmation{},
	}
}

func (client *HydrophoneMockClient) GetPendingInvitations(userID string, authToken string) ([]models.Confirmation, error) {
	if client.MockedError != nil {
		return nil, client.MockedError
	}
	return client.MockedConfirms, nil
}

//TODO: refactor methods above to use testify like bellow

func (client *HydrophoneMockClient) GetPendingSignup(userId string, authToken string) (*models.Confirmation, error) {
	args := client.Called(userId, authToken)
	return args.Get(0).(*models.Confirmation), args.Error(1)
}

func (client *HydrophoneMockClient) CancelSignup(confirm models.Confirmation, authToken string) error {
	client.Called(confirm, authToken)
	return nil
}
