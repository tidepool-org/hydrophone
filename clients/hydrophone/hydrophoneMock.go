package hydrophone

import (
	"github.com/mdblp/hydrophone/models"
)

type HydrophoneMockClient struct {
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
