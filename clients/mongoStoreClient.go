package clients

import (
	"./../models"
	"github.com/tidepool-org/go-common/clients/mongo"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"log"
)

const (
	NOTIFICATIONS_COLLECTION = "notifications"
)

type MongoStoreClient struct {
	session        *mgo.Session
	notificationsC *mgo.Collection
}

func NewMongoStoreClient(config *mongo.Config) *MongoStoreClient {

	mongoSession, err := mongo.Connect(config)
	if err != nil {
		log.Fatal(err)
	}

	return &MongoStoreClient{
		session:        mongoSession,
		notificationsC: mongoSession.DB("").C(NOTIFICATIONS_COLLECTION),
	}
}

func (d MongoStoreClient) Close() {
	log.Println("Close the session")
	d.session.Close()
	return
}

func (d MongoStoreClient) Ping() error {
	// do we have a store session
	if err := d.session.Ping(); err != nil {
		return err
	}
	return nil
}

func (d MongoStoreClient) UpsertNotification(notification *models.Notification) error {

	// if the user already exists we update otherwise we add
	if _, err := d.notificationsC.Upsert(bson.M{"_id": notification.Id}, notification); err != nil {
		return err
	}
	return nil
}

func (d MongoStoreClient) FindNotification(notification *models.Notification) (result *models.Notification, err error) {

	if notification.Id != "" {
		if err = d.notificationsC.Find(bson.M{"_id": notification.Id}).One(&result); err != nil {
			return result, err
		}
	}

	return result, nil
}

func (d MongoStoreClient) RemoveNotification(notification *models.Notification) error {
	if err := d.notificationsC.Remove(bson.M{"_id": notification.Id}); err != nil {
		return err
	}
	return nil
}
