package hydrophone

import (
	"errors"
	"fmt"
	"github.com/mdblp/hydrophone/models"
	"testing"
)

func TestMock(t *testing.T) {

	client := NewMock()
	testToken := "1.2.3"
	testUserID := "123"
	// Default behavior no error empty result
	invites, err := client.GetPendingInvitations(testUserID, testToken)
	if err != nil {
		t.Errorf("Failed when mock is initialized, GetPendingInvitations should return nil error.\nExpected nil got %v\n", err)
	}
	if len(invites) > 0 {
		t.Errorf("Failed when mock is initialized, GetPendingInvitations should return empty invites.\nExpected [] got %v\n", invites)
	}
	// Error behavior
	mockError := errors.New("hydrophone error")
	client.MockedError = mockError
	invites, err = client.GetPendingInvitations(testUserID, testToken)
	if err == nil {
		t.Errorf("Failed when mocked with an error, GetPendingInvitations should return an error.\nEexpected %v got nil\n", mockError)
	}
	if fmt.Sprintf("%v", err) != "hydrophone error" {
		t.Errorf("Failed when mocked with an error, GetPendingInvitations should return an error.\nExpected %v got %v\n", mockError, err)
	}
	if invites != nil {
		t.Errorf("Failed when mocked with an error, GetPendingInvitations should return nil result.\nExpected nil got %v\n", invites)
	}
	// Result behavior
	client.MockedError = nil
	client.MockedConfirms = []models.Confirmation{
		{
			Key:       "confirm-key-1",
			Type:      models.TypePasswordReset,
			Role:      "member",
			Email:     "test@test.fr",
			CreatorId: "123",
			Status:    models.StatusPending,
		},
		{
			Key:       "confirm-key-2",
			Type:      models.TypeMedicalTeamInvite,
			Role:      "member",
			Email:     "test2@test2.fr",
			CreatorId: "123",
			UserId:    "1000",
			Status:    models.StatusPending,
		},
	}
	invites, err = client.GetPendingInvitations(testUserID, testToken)
	if err != nil {
		t.Errorf("Failed when mocked with results, GetPendingInvitations should return nil error.\nExpected nil got %v", err)
	}
	if invites == nil {
		t.Errorf("Failed when mocked with an results, GetPendingInvitations should return results.\nExpected %v got nil\n", invites)
	}
	if len(invites) != len(client.MockedConfirms) || invites[0].Key != client.MockedConfirms[0].Key || invites[1].Key != client.MockedConfirms[1].Key {
		t.Errorf("Failed when mocked with an results, GetPendingInvitations should return mocked results.\nExpected %v got %v\n", client.MockedConfirms, invites)
	}
}
