package clients

import (
	"context"
	"errors"
	"time"

	"github.com/tidepool-org/hydrophone/models"
	"go.mongodb.org/mongo-driver/mongo"
)

type MockStoreClient struct {
	doBad      bool
	returnNone bool
	now        time.Time
}

func NewMockStoreClient(returnNone, doBad bool) *MockStoreClient {
	return &MockStoreClient{doBad: doBad, returnNone: returnNone, now: time.Now()}
}

func (d *MockStoreClient) Close() error {
	return nil
}

func (d *MockStoreClient) Ping() error {
	if d.doBad {
		return errors.New("Session failure")
	}
	return nil
}
func (d *MockStoreClient) PingOK() bool {
	return !d.doBad
}
func (d *MockStoreClient) Collection(collectionName string, databaseName ...string) *mongo.Collection {
	return nil
}
func (d *MockStoreClient) WaitUntilStarted() {}
func (d *MockStoreClient) Start()            {}
func (d *MockStoreClient) UpsertConfirmation(ctx context.Context, notification *models.Confirmation) error {
	if d.doBad {
		return errors.New("UpsertConfirmation failure")
	}
	if notification.Email == "patient@myemail.com" && notification.ShortKey == "" {
		return errors.New("password reset for a patient should contain a short key")
	}
	if notification.Email == "clinic@myemail.com" && notification.ShortKey != "" {
		return errors.New("password reset for a clinician should NOT contain a short key")
	}
	if notification.Email == "patient@myemail.com" && notification.Key != "" && notification.ShortKey != "" {
		return nil
	}
	if notification.Email == "clinic@myemail.com" && notification.Key != "" && notification.ShortKey == "" {
		return nil
	}
	return nil
}

func (d *MockStoreClient) FindConfirmation(ctx context.Context, notification *models.Confirmation) (result *models.Confirmation, err error) {
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
	if notification.Email == "email.resend@address.org" {
		notification.TemplateName = models.TemplateNameSignup
	}
	if notification.ShortKey == "12345678" {
		thirtyminutes, _ := time.ParseDuration("30m")
		notification.Created = time.Now().Add(thirtyminutes) // created 30 minutes ago
	} else {
		notification.Created = time.Now().AddDate(0, 0, -3) // created three days ago
	}
	if notification.ShortKey == "11111111" {
		return nil, nil
	}
	return notification, nil
}

func (d *MockStoreClient) FindConfirmations(ctx context.Context, confirmation *models.Confirmation, statuses ...models.Status) (results []*models.Confirmation, err error) {
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

func (d *MockStoreClient) RemoveConfirmation(ctx context.Context, notification *models.Confirmation) error {
	if d.doBad {
		return errors.New("RemoveConfirmation failure")
	}
	return nil
}
