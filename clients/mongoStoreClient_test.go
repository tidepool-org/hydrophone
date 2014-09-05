package clients

import (
	"./../models"
	"encoding/json"
	"github.com/tidepool-org/go-common/clients/mongo"
	"io/ioutil"
	"labix.org/v2/mgo"
	"testing"
	"time"
)

func TestMongoStoreTokenOperations(t *testing.T) {

	type Config struct {
		Mongo *mongo.Config `json:"mongo"`
	}

	var (
		config       Config
		notification = &models.Notification{
			Id:       "123456789",
			Key:      "notification_type/abcdefghijklmn_Hs4we",
			Content:  "content from template",
			ToUser:   "test@user.org",
			FromUser: "",
			Created:  time.Now(),
			Sent:     time.Now(),
		}
	)

	if jsonConfig, err := ioutil.ReadFile("../config/server.json"); err == nil {

		if err := json.Unmarshal(jsonConfig, &config); err != nil {
			t.Fatalf("We could not load the config ", err)
		}

		mc := NewMongoStoreClient(config.Mongo)

		/*
		 * INIT THE TEST - we use a clean copy of the collection before we start
		 */

		//drop and don't worry about any errors
		mc.notificationsC.DropCollection()

		if err := mc.notificationsC.Create(&mgo.CollectionInfo{}); err != nil {
			t.Fatalf("We couldn't created the users collection for these tests ", err)
		}

		/*
		 * THE TESTS
		 */

		if err := mc.UpsertNotification(notification); err != nil {
			t.Fatalf("we could not save the notification %v", err)
		}

		if found, err := mc.FindNotification(notification); err == nil {
			if found.Id == "" {
				t.Fatalf("the token string isn't included %v", found)
			}
		} else {
			t.Fatalf("no token was returned when it should have been - err[%v]", err)
		}

		if err := mc.RemoveNotification(notification); err != nil {
			t.Fatalf("we could not remove the token %v", err)
		}

		if token, err := mc.FindNotification(notification); err == nil {
			if token != nil {
				t.Fatalf("the token has been removed so we shouldn't find it %v", token)
			}
		}

	} else {
		t.Fatalf("wtf - failed parsing the config %v", err)
	}
}
