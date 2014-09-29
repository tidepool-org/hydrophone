package clients

import (
	"log"

	"./../models"
	"github.com/tidepool-org/go-common/clients/mongo"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
)

const (
	CONFIRMATIONS_COLLECTION = "confirmations"
)

type MongoStoreClient struct {
	session        *mgo.Session
	confirmationsC *mgo.Collection
}

func NewMongoStoreClient(config *mongo.Config) *MongoStoreClient {

	mongoSession, err := mongo.Connect(config)
	if err != nil {
		log.Fatal(err)
	}

	return &MongoStoreClient{
		session:        mongoSession,
		confirmationsC: mongoSession.DB("").C(CONFIRMATIONS_COLLECTION),
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

func (d MongoStoreClient) UpsertConfirmation(confirmation *models.Confirmation) error {

	// if the user already exists we update otherwise we add
	if _, err := d.confirmationsC.Upsert(bson.M{"key": confirmation.Key}, confirmation); err != nil {
		return err
	}
	return nil
}

func (d MongoStoreClient) FindConfirmation(confirmation *models.Confirmation) (result *models.Confirmation, err error) {

	var query bson.M = bson.M{}

	if confirmation.ToEmail != "" {
		query["toemail"] = confirmation.ToEmail
	}
	if confirmation.Key != "" {
		query["key"] = confirmation.Key
	}
	if string(confirmation.Status) != "" {
		query["status"] = confirmation.Status
	}
	if string(confirmation.Type) != "" {
		query["type"] = confirmation.Type
	}
	if confirmation.CreatorId != "" {
		query["creatorId"] = confirmation.CreatorId
	}
	if confirmation.ToUser != "" {
		query["touser"] = confirmation.ToUser
	}

	if err = d.confirmationsC.Find(query).One(&result); err != nil {
		return result, err
	}

	return result, nil
}

func (d MongoStoreClient) FindConfirmationByKey(key string) (result *models.Confirmation, err error) {

	if key != "" {
		if err = d.confirmationsC.Find(bson.M{"key": key}).One(&result); err != nil {
			return result, err
		}
	}

	return result, nil
}

func (d MongoStoreClient) FindConfirmations(userEmail, creatorId string, status models.Status) (results []*models.Confirmation, err error) {

	var query bson.M

	if userEmail != "" {
		query = bson.M{"toemail": userEmail, "status": status}
	} else if creatorId != "" {
		query = bson.M{"creatorId": creatorId, "status": status}
	}

	if err = d.confirmationsC.Find(query).All(&results); err != nil {
		return results, err
	}
	return results, nil
}

func (d MongoStoreClient) RemoveConfirmation(confirmation *models.Confirmation) error {
	if err := d.confirmationsC.Remove(bson.M{"key": confirmation.Key}); err != nil {
		return err
	}
	return nil
}
