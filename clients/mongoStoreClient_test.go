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
		confirmation, _ = models.NewConfirmation(models.TypePasswordReset, "user@test.org", "123.456")
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

		/*
		 * THE TESTS
		 */

		if err := mc.UpsertConfirmation(confirmation); err != nil {
			t.Fatalf("we could not save the con %v", err)
		}

		if found, err := mc.FindConfirmation(confirmation); err == nil {
			if found.Id == "" {
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

	} else {
		t.Fatalf("wtf - failed parsing the config %v", err)
	}
}
