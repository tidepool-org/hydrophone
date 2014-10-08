package clients

import (
	"./../models"
	"encoding/json"
	"github.com/tidepool-org/go-common/clients/mongo"
	"io/ioutil"
	"labix.org/v2/mgo"
	"testing"
)

func TestMongoStoreConfirmationOperations(t *testing.T) {

	type Config struct {
		Mongo *mongo.Config `json:"mongo"`
	}

	var (
		config          Config
		confirmation, _ = models.NewConfirmation(models.TypePasswordReset, "123.456")
	)

	if jsonConfig, err := ioutil.ReadFile("../config/server.json"); err == nil {

		if err := json.Unmarshal(jsonConfig, &config); err != nil {
			t.Fatalf("We could not load the config ", err)
		}

		mc := NewMongoStoreClient(config.Mongo)

		/*
		 * INIT THE TEST - we use a clean copy of the collection before we start
		 */

		//drop it like its hot
		mc.confirmationsC.DropCollection()

		if err := mc.confirmationsC.Create(&mgo.CollectionInfo{}); err != nil {
			t.Fatalf("We couldn't created the users collection for these tests ", err)
		}

		//The basics
		if err := mc.UpsertConfirmation(confirmation); err != nil {
			t.Fatalf("we could not save the con %v", err)
		}

		if found, err := mc.FindConfirmation(confirmation); err == nil {
			if found.Key == "" {
				t.Fatalf("the confirmation string isn't included %v", found)
			}
		} else {
			t.Fatalf("no confirmation was returned when it should have been - err[%v]", err)
		}

		if err := mc.RemoveConfirmation(confirmation); err != nil {
			t.Fatalf("we could not remove the confirmation %v", err)
		}

		if confirmation, err := mc.FindConfirmation(confirmation); err == nil {
			if confirmation != nil {
				t.Fatalf("the confirmation has been removed so we shouldn't find it %v", confirmation)
			}
		}

		//Find with other statuses
		const fromUser, toUser, toEmail = "999.111", "312.123", "some@email.org"
		c1, _ := models.NewConfirmation(models.TypeCareteamInvite, fromUser)
		c1.UserId = toUser
		c1.UpdateStatus(models.StatusDeclined)
		mc.UpsertConfirmation(c1)

		c2, _ := models.NewConfirmation(models.TypeCareteamInvite, fromUser)
		c2.Email = toEmail
		c2.UpdateStatus(models.StatusCompleted)
		mc.UpsertConfirmation(c2)

		if confirmations, err := mc.ConfirmationsFromUser(fromUser, models.StatusDeclined, models.StatusCompleted); err == nil {
			if len(confirmations) != 2 {
				t.Fatalf("we should have found 2 confirmations %v", confirmations)
			}
			if confirmations[0].Status != models.StatusCompleted && confirmations[0].Status != models.StatusDeclined {
				t.Fatalf("status invalid: %s", confirmations[0].Status)
			}
			if confirmations[1].Status != models.StatusCompleted && confirmations[1].Status != models.StatusDeclined {
				t.Fatalf("status invalid: %s", confirmations[1].Status)
			}
		}

		if confirmations, err := mc.ConfirmationsToUser(toUser, models.StatusDeclined, models.StatusCompleted); err == nil {
			if len(confirmations) != 1 {
				t.Fatalf("we should have found 1 confirmations %v", confirmations)
			}
			if confirmations[0].UserId != toUser {
				t.Fatalf("should be for user: %s", toUser)
			}
			if confirmations[0].Status != models.StatusCompleted && confirmations[0].Status != models.StatusDeclined {
				t.Fatalf("status invalid: %s", confirmations[0].Status)
			}
		}

		if confirmations, err := mc.ConfirmationsToEmail(toEmail, models.StatusDeclined, models.StatusCompleted); err == nil {
			if len(confirmations) != 1 {
				t.Fatalf("we should have found 1 confirmations %v", confirmations)
			}
			if confirmations[0].Email != toEmail {
				t.Fatalf("should be for email: %s", toEmail)
			}
			if confirmations[0].Status != models.StatusCompleted && confirmations[0].Status != models.StatusDeclined {
				t.Fatalf("status invalid: %s", confirmations[0].Status)
			}
		}

	} else {
		t.Fatalf("wtf - failed parsing the config %v", err)
	}
}
