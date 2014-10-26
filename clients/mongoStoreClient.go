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
	if _, err := d.confirmationsC.Upsert(bson.M{"_id": confirmation.Key}, confirmation); err != nil {
		return err
	}
	return nil
}

func (d MongoStoreClient) FindConfirmation(confirmation *models.Confirmation) (result *models.Confirmation, err error) {

	var query bson.M = bson.M{}

	if confirmation.Email != "" {
		query["email"] = confirmation.Email
	}
	if confirmation.Key != "" {
		query["_id"] = confirmation.Key
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
	if confirmation.UserId != "" {
		query["userId"] = confirmation.UserId
	}

	if err = d.confirmationsC.Find(query).One(&result); err != nil {
		return result, err
	}

	return result, nil
}

func (d MongoStoreClient) FindConfirmationByKey(key string) (result *models.Confirmation, err error) {

	if key != "" {
		if err = d.confirmationsC.Find(bson.M{"_id": key}).One(&result); err != nil {
			return result, err
		}
	}

	return result, nil
}

func (d MongoStoreClient) ConfirmationsToUser(fromId, toId, toEmail string, statuses ...models.Status) (results []*models.Confirmation, err error) {

	query := []bson.M{}

	if toId != "" && fromId != "" {
		query = append(query, bson.M{"creatorId": fromId, "userId": toId, "status": bson.M{"$in": statuses}})
	}
	if toEmail != "" && fromId != "" {
		query = append(query, bson.M{"creatorId": fromId, "email": toEmail, "status": bson.M{"$in": statuses}})
	}
	return d.queryConfirmations(bson.M{"$or": query})
}

func (d MongoStoreClient) ConfirmationsFromUser(creatorId string, statuses ...models.Status) (results []*models.Confirmation, err error) {
	query := bson.M{"creatorId": creatorId, "status": bson.M{"$in": statuses}}
	return d.queryConfirmations(query)
}

func (d MongoStoreClient) queryConfirmations(query interface{}) (results []*models.Confirmation, err error) {
	if err = d.confirmationsC.Find(query).All(&results); err != nil {
		return results, err
	}
	return results, nil
}

func (d MongoStoreClient) RemoveConfirmation(confirmation *models.Confirmation) error {
	if err := d.confirmationsC.Remove(bson.M{"_id": confirmation.Key}); err != nil {
		return err
	}
	return nil
}
