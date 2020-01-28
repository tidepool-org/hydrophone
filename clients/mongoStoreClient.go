package clients

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	tpMongo "github.com/tidepool-org/go-common/clients/mongo"
	"github.com/tidepool-org/hydrophone/models"
)

const (
	confirmationsCollectionName = "confirmations"
)

// MongoStoreClient - Mongo Storage Client
type MongoStoreClient struct {
	client   *mongo.Client
	context  context.Context
	database string
}

// NewMongoStoreClient creates a new MongoStoreClient
func NewMongoStoreClient(config *tpMongo.Config) *MongoStoreClient {
	connectionString, err := config.ToConnectionString()
	if err != nil {
		log.Fatal(fmt.Sprintf("Invalid MongoDB configuration: %s", err))
	}

	clientOptions := options.Client().ApplyURI(connectionString)
	mongoClient, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(fmt.Sprintf("Invalid MongoDB connection string: %s", err))
	}

	return &MongoStoreClient{
		client:   mongoClient,
		context:  context.Background(),
		database: config.Database,
	}
}

// WithContext returns a shallow copy of c with its context changed
// to ctx. The provided ctx must be non-nil.
func (c *MongoStoreClient) WithContext(ctx context.Context) StoreClient {
	if ctx == nil {
		panic("nil context")
	}
	c2 := new(MongoStoreClient)
	*c2 = *c
	c2.context = ctx
	return c2
}

// EnsureIndexes exist for the MongoDB collection. EnsureIndexes uses the Background() context, in order
// to pass back the MongoDB errors, rather than any context errors.
// TODO: There could be more indexes here for performance reasons.
// Current performance as of 2020-01 is sufficient.
func (c *MongoStoreClient) EnsureIndexes() error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "type", Value: 1}, {Key: "status", Value: 1}},
			Options: options.Index().
				SetBackground(true),
		},
	}

	if _, err := confirmationsCollection(c).Indexes().CreateMany(context.Background(), indexes); err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

// wrapper function for consistent access to the collection
func confirmationsCollection(c *MongoStoreClient) *mongo.Collection {
	return c.client.Database(c.database).Collection(confirmationsCollectionName)
}

// Ping the MongoDB database
func (c *MongoStoreClient) Ping() error {
	// do we have a store session
	return c.client.Ping(c.context, nil)
}

// Disconnect from the MongoDB database
func (c *MongoStoreClient) Disconnect() error {
	return c.client.Disconnect(c.context)
}

// UpsertConfirmation updates an existing confirmation, or inserts a new one if not already present.
func (c *MongoStoreClient) UpsertConfirmation(confirmation *models.Confirmation) error {
	opts := options.FindOneAndUpdate().SetUpsert(true)
	result := confirmationsCollection(c).FindOneAndUpdate(c.context, bson.M{"_id": confirmation.Key}, bson.D{{Key: "$set", Value: confirmation}}, opts)
	if result.Err() != mongo.ErrNoDocuments {
		return result.Err()
	}
	return nil
}

// FindConfirmation - find and return an existing confirmation
func (c *MongoStoreClient) FindConfirmation(confirmation *models.Confirmation) (result *models.Confirmation, err error) {
	var query bson.M = bson.M{}

	if confirmation.Email != "" {
		// case insensitive match
		// TODO: should use an index with collation, not a case-insensitive regex, since that can't use an index
		// However, we need MongoDB 3.2 to do this. See https://tidepool.atlassian.net/browse/BACK-1133
		query["email"] = bson.M{"$regex": primitive.Regex{Pattern: fmt.Sprintf("^%s$", regexp.QuoteMeta(confirmation.Email)), Options: "i"}}
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

	opts := options.FindOne().SetSort(bson.D{{"created", -1}})

	if err = confirmationsCollection(c).FindOne(c.context, query, opts).Decode(&result); err != nil && err != mongo.ErrNoDocuments {
		log.Printf("FindConfirmation: something bad happened [%v]", err)
		return result, err
	}

	return result, nil
}

// FindConfirmations - find and return existing confirmations
func (c *MongoStoreClient) FindConfirmations(confirmation *models.Confirmation, statuses ...models.Status) (results []*models.Confirmation, err error) {
	var query bson.M = bson.M{}

	if confirmation.Email != "" {
		// case insensitive match
		// TODO: should use an index with collation, not a case-insensitive regex, since that can't use an index
		// However, we need MongoDB 3.2 to do this. See https://tidepool.atlassian.net/browse/BACK-1133
		query["email"] = bson.M{"$regex": primitive.Regex{Pattern: fmt.Sprintf("^%s$", regexp.QuoteMeta(confirmation.Email)), Options: "i"}}
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

	opts := options.Find().SetSort(bson.D{{"created", -1}})
	cursor, err := confirmationsCollection(c).Find(c.context, query, opts)
	if err != nil {
		return nil, err
	}

	if err = cursor.All(c.context, &results); err != nil {
		log.Printf("FindConfirmations: something bad happened [%v]", err)
		return results, err
	}
	return results, nil
}

// RemoveConfirmation - Remove a confirmation from the database
func (c *MongoStoreClient) RemoveConfirmation(confirmation *models.Confirmation) error {
	result := confirmationsCollection(c).FindOneAndDelete(c.context, bson.M{"_id": confirmation.Key})
	if result.Err() != mongo.ErrNoDocuments {
		return result.Err()
	}
	return nil
}
