package clients

import (
	"context"
	"errors"
	"time"

	"github.com/mdblp/hydrophone/models"
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
	if notification.Key == "signupkey" && notification.Type != models.TypeSignUp {
		return nil, nil
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
	if notification.Key == "key.to.be.dismissed" {
		notification.Status = "pending"
	}
	if notification.Key == "patient.key.to.be.dismissed" {
		notification.Type = "medicalteam_patient_invitation"
		notification.Status = "pending"
	}
	if notification.Key == "invite.wrong.type" {
		notification.Status = "pending"
		notification.Type = "a.wrong.type"
		if notification.Team == nil {
			notification.Team = &models.Team{}
		}
		notification.Team.ID = "123456"
		notification.UserId = "123.456.789"
	}
	if notification.Key == "medicalteam.invite.member" {
		notification.Status = "pending"
		notification.Type = "medicalteam_invitation"
		if notification.Team == nil {
			notification.Team = &models.Team{}
		}
		notification.Team.ID = "123456"
		notification.UserId = "UID123"
	}
	if notification.Key == "medicalteam.invite.wrong.member" {
		notification.Status = "pending"
		notification.Type = "medicalteam_invitation"
		if notification.Team == nil {
			notification.Team = &models.Team{}
		}
		notification.Team.ID = "123456"
		notification.UserId = "not.my.id"
	}
	if notification.Key == "medicalteam.invite.patient" {
		notification.Status = "pending"
		notification.Type = "medicalteam_patient_invitation"
		if notification.Team == nil {
			notification.Team = &models.Team{}
		}
		notification.Team.ID = "123456"
		notification.UserId = "UID123"
	}
	if notification.Key == "invalid.key" {
		notification.Status = ""
		notification.Type = ""
		if notification.Team == nil {
			notification.Team = &models.Team{}
		}
		notification.Team.ID = ""
		notification.UserId = ""
	}
	if notification.Key == "key.does.not.exist" {
		return nil, nil
	}
	if notification.Key == "any.invite.invalid.key" {
		return nil, nil
	}
	if notification.Key == "any.invite.completed.key" {
		notification.Status = "completed"
	}
	if notification.Key == "any.invite.pending.do.admin" {
		notification.Status = "pending"
		notification.Type = "medicalteam_do_admin"
		notification.UserId = "UID123"
	}
	if notification.Key == "any.invite.pending.remove" {
		notification.Status = "pending"
		notification.Type = "medicalteam_remove"
		notification.UserId = "UID123"
	}
	return notification, nil
}

func (d *MockStoreClient) FindConfirmations(ctx context.Context, confirmation *models.Confirmation, statuses []models.Status, types []models.Type) (results []*models.Confirmation, err error) {
	if d.doBad {
		return nil, errors.New("FindConfirmation failure")
	}
	if d.returnNone {
		return nil, nil
	}

	confirmation.Created = time.Now().AddDate(0, 0, -3) // created three days ago
	confirmation.Context = []byte(`{"view":{}, "note":{}}`)
	if len(statuses) == 1 {
		confirmation.UpdateStatus(statuses[0])
	}

	return []*models.Confirmation{confirmation}, nil
}

func (d *MockStoreClient) RemoveConfirmation(ctx context.Context, notification *models.Confirmation) error {
	if d.doBad {
		return errors.New("RemoveConfirmation failure")
	}
	return nil
}
