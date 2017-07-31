package clients

import (
	"testing"
	"time"

	"labix.org/v2/mgo"

	"github.com/tidepool-org/go-common/clients/mongo"
	"github.com/tidepool-org/hydrophone/models"
)

func TestMongoStoreConfirmationOperations(t *testing.T) {

	confirmation, _ := models.NewConfirmation(models.TypePasswordReset, models.TemplateNamePasswordReset, "123.456")
	confirmation.Email = "test@test.com"

	doesNotExist, _ := models.NewConfirmation(models.TypePasswordReset, models.TemplateNamePasswordReset, "123.456")

	testingConfig := &mongo.Config{ConnectionString: "mongodb://localhost/confirm_test"}

	mc := NewMongoStoreClient(testingConfig)

	/*
	 * INIT THE TEST - we use a clean copy of the collection before we start
	 */

	//drop it like its hot
	cpy := mc.session.Copy()
	defer cpy.Close()

	mgoConfirmationsCollection(cpy).DropCollection()

	if err := mgoConfirmationsCollection(cpy).Create(&mgo.CollectionInfo{}); err != nil {
		t.Fatalf("We couldn't created the users collection for these tests ", err)
	}

	//The basics
	//+++++++++++++++++++++++++++
	if err := mc.UpsertConfirmation(confirmation); err != nil {
		t.Fatalf("we could not save the con %v", err)
	}

	if found, err := mc.FindConfirmation(confirmation); err == nil {
		if found == nil {
			t.Fatalf("the confirmation was not found")
		}
		if found.Key == "" {
			t.Fatalf("the confirmation string isn't included %v", found)
		}
	} else {
		t.Fatalf("no confirmation was returned when it should have been - err[%v]", err)
	}

	// Uppercase the email and try again (detect case sensitivity)
	confirmation.Email = "TEST@TEST.COM"
	if found, err := mc.FindConfirmation(confirmation); err == nil {
		if found == nil {
			t.Fatalf("the uppercase confirmation was not found")
		}
		if found.Key == "" {
			t.Fatalf("the confirmation string isn't included %v", found)
		}
	} else {
		t.Fatalf("no confirmation was returned when it should have been - err[%v]", err)
	}

	//when the conf doesn't exist
	if found, err := mc.FindConfirmation(doesNotExist); err == nil && found != nil {
		t.Fatalf("there should have been no confirmation found [%v]", found)
	} else if err != nil {
		t.Fatalf("and error was returned when it should not have been err[%v]", err)
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
	const fromUser, toUser, toEmail, toOtherEmail = "999.111", "312.123", "some@email.org", "some@other.org"
	c1, _ := models.NewConfirmation(models.TypeCareteamInvite, models.TemplateNameCareteamInvite, fromUser)
	c1.UserId = toUser
	c1.Email = toEmail
	c1.UpdateStatus(models.StatusDeclined)
	mc.UpsertConfirmation(c1)

	// Sleep some so the second confirmation created time is after the first confirmation created time
	time.Sleep(time.Second)

	c2, _ := models.NewConfirmation(models.TypeCareteamInvite, models.TemplateNameCareteamInvite, fromUser)
	c2.Email = toOtherEmail
	c2.UpdateStatus(models.StatusCompleted)
	mc.UpsertConfirmation(c2)

	searchForm := &models.Confirmation{CreatorId: fromUser}

	if confirmations, err := mc.FindConfirmations(searchForm, models.StatusDeclined, models.StatusCompleted); err == nil {
		if len(confirmations) != 2 {
			t.Fatalf("we should have found 2 confirmations %v", confirmations)
		}

		t1 := confirmations[0].Created
		t2 := confirmations[1].Created

		if !t1.After(t2) {
			t.Fatalf("the newest confirmtion should be first %v", confirmations)
		}

		if confirmations[0].Email != toOtherEmail {
			t.Fatalf("email invalid: %s", confirmations[0].Email)
		}
		if confirmations[0].Status != models.StatusCompleted && confirmations[0].Status != models.StatusDeclined {
			t.Fatalf("status invalid: %s", confirmations[0].Status)
		}
		if confirmations[1].Email != toEmail {
			t.Fatalf("email invalid: %s", confirmations[1].Email)
		}
		if confirmations[1].Status != models.StatusCompleted && confirmations[1].Status != models.StatusDeclined {
			t.Fatalf("status invalid: %s", confirmations[1].Status)
		}
	}
	searchToOtherEmail := &models.Confirmation{CreatorId: fromUser, Email: toOtherEmail}
	//only email address
	if confirmations, err := mc.FindConfirmations(searchToOtherEmail, models.StatusDeclined, models.StatusCompleted); err == nil {
		if len(confirmations) != 1 {
			t.Fatalf("we should have found 1 confirmations %v", confirmations)
		}
		if confirmations[0].Email != toOtherEmail {
			t.Fatalf("should be for email: %s", toOtherEmail)
		}
		if confirmations[0].Status != models.StatusCompleted && confirmations[0].Status != models.StatusDeclined {
			t.Fatalf("status invalid: %s", confirmations[0].Status)
		}
	}
	searchToEmail := &models.Confirmation{CreatorId: fromUser, Email: toEmail}
	//with both userid and email address
	if confirmations, err := mc.FindConfirmations(searchToEmail, models.StatusDeclined, models.StatusCompleted); err == nil {
		if len(confirmations) != 1 {
			t.Fatalf("we should have found 1 confirmations %v", confirmations)
		}
		if confirmations[0].UserId != toUser {
			t.Fatalf("should be for user: %s", toUser)
		}
		if confirmations[0].Email != toEmail {
			t.Fatalf("should be for email: %s", toEmail)
		}
		if confirmations[0].Status != models.StatusCompleted && confirmations[0].Status != models.StatusDeclined {
			t.Fatalf("status invalid: %s", confirmations[0].Status)
		}
	}
}
