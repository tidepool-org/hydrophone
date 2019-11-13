package clients

import (
	"fmt"
	"log"
	"regexp"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"

	"github.com/tidepool-org/go-common/clients/mongo"
	"github.com/tidepool-org/hydrophone/models"
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

//warpper function for consistent access to the collection
func mgoConfirmationsCollection(cpy *mgo.Session) *mgo.Collection {
	return cpy.DB("").C(CONFIRMATIONS_COLLECTION)
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

	cpy := d.session.Copy()
	defer cpy.Close()

	// if the user already exists we update otherwise we add
	if _, err := mgoConfirmationsCollection(cpy).Upsert(bson.M{"_id": confirmation.Key}, confirmation); err != nil {
		return err
	}
	return nil
}

func (d MongoStoreClient) FindConfirmation(confirmation *models.Confirmation) (result *models.Confirmation, err error) {

	var query bson.M = bson.M{}

	if confirmation.Email != "" {
		query["email"] = bson.M{"$regex": bson.RegEx{fmt.Sprintf("^%s$", regexp.QuoteMeta(confirmation.Email)), "i"}}
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

	cpy := d.session.Copy()
	defer cpy.Close()

	if err = mgoConfirmationsCollection(cpy).Find(query).Sort("-created").One(&result); err != nil && err != mgo.ErrNotFound {
		log.Printf("FindConfirmation: something bad happened [%v]", err)
		return result, err
	}

	return result, nil
}

func (d MongoStoreClient) FindConfirmations(confirmation *models.Confirmation, statuses ...models.Status) (results []*models.Confirmation, err error) {

	var query bson.M = bson.M{}

	if confirmation.Email != "" {
		query["email"] = bson.M{"$regex": bson.RegEx{fmt.Sprintf("^%s$", regexp.QuoteMeta(confirmation.Email)), "i"}}
	}
	if confirmation.Key != "" {
		query["_id"] = confirmation.Key
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

	if len(statuses) > 0 {
		query["status"] = bson.M{"$in": statuses}
	}

	cpy := d.session.Copy()
	defer cpy.Close()

	if err = mgoConfirmationsCollection(cpy).Find(query).Sort("-created").All(&results); err != nil && err != mgo.ErrNotFound {
		log.Printf("FindConfirmations: something bad happened [%v]", err)
		return results, err
	}
	return results, nil
}

func (d MongoStoreClient) RemoveConfirmation(confirmation *models.Confirmation) error {

	cpy := d.session.Copy()
	defer cpy.Close()

	if err := mgoConfirmationsCollection(cpy).Remove(bson.M{"_id": confirmation.Key}); err != nil {
		return err
	}
	return nil
}
