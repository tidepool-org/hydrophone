package clients

import (
	"context"
	"fmt"
	"regexp"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
	"go.uber.org/zap"

	tpMongo "github.com/tidepool-org/go-common/clients/mongo"
	"github.com/tidepool-org/hydrophone/models"
)

const (
	confirmationsCollectionName = "confirmations"
)

// MongoStoreClient - Mongo Storage Client
type MongoStoreClient struct {
	client   *mongo.Client
	database string
	log      *zap.SugaredLogger
}

// NewMongoStoreClient creates a new MongoStoreClient
func NewMongoStoreClient(config *tpMongo.Config, log *zap.SugaredLogger) (*MongoStoreClient, error) {
	connectionString, err := config.ToConnectionString()
	if err != nil {
		return nil, errors.Wrap(err, "invalid MongoDB configuration")
	}

	clientOptions := options.Client().ApplyURI(connectionString)
	mongoClient, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, errors.Wrap(err, "invalid MongoDB connection string")
	}

	return &MongoStoreClient{
		client:   mongoClient,
		database: config.Database,
		log:      log,
	}, nil
}

// EnsureIndexes exist for the MongoDB collection. EnsureIndexes uses the Background() context, in order
// to pass back the MongoDB errors, rather than any context errors.
// TODO: There could be more indexes here for performance reasons.
// Current performance as of 2020-01 is sufficient.
func (c *MongoStoreClient) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "type", Value: 1}, {Key: "status", Value: 1}},
			Options: options.Index().
				SetBackground(true),
		},
	}

	if _, err := confirmationsCollection(c).Indexes().CreateMany(ctx, indexes); err != nil {
		c.log.With(zap.Error(err)).Fatal("creating indexes")
	}

	return nil
}

func mongoConfigProvider() (tpMongo.Config, error) {
	var config tpMongo.Config
	err := envconfig.Process("", &config)
	if err != nil {
		return tpMongo.Config{}, err
	}
	return config, nil
}

func mongoStoreProvider(config tpMongo.Config, log *zap.SugaredLogger) (StoreClient, error) {
	return NewMongoStoreClient(&config, log)
}

// MongoModule for dependency injection
var MongoModule = fx.Options(fx.Provide(mongoConfigProvider, mongoStoreProvider))

// wrapper function for consistent access to the collection
func confirmationsCollection(c *MongoStoreClient) *mongo.Collection {
	return c.client.Database(c.database).Collection(confirmationsCollectionName)
}

// Ping the MongoDB database
func (c *MongoStoreClient) Ping(ctx context.Context) error {
	// do we have a store session
	return c.client.Ping(ctx, nil)
}

// Disconnect from the MongoDB database
func (c *MongoStoreClient) Disconnect(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}

// UpsertConfirmation updates an existing confirmation, or inserts a new one if not already present.
func (c *MongoStoreClient) UpsertConfirmation(ctx context.Context, confirmation *models.Confirmation) error {
	opts := options.FindOneAndUpdate().SetUpsert(true)
	result := confirmationsCollection(c).FindOneAndUpdate(ctx,
		bson.M{"_id": confirmation.Key}, bson.D{{Key: "$set", Value: confirmation}}, opts)
	if result.Err() != mongo.ErrNoDocuments {
		return result.Err()
	}
	return nil
}

// FindConfirmation - find and return an existing confirmation
func (c *MongoStoreClient) FindConfirmation(ctx context.Context, confirmation *models.Confirmation) (result *models.Confirmation, err error) {
	var query bson.M = bson.M{}

	if confirmation.Email != "" {
		// case insensitive match
		// TODO: should use an index with collation, not a case-insensitive regex, since that can't use an index
		// However, we need MongoDB 3.2 to do this. See https://tidepool.atlassian.net/browse/BACK-1133
		// Collated index on type+email
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
	if confirmation.ClinicId != "" {
		query["clinicId"] = confirmation.ClinicId
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "created", Value: -1}})

	if err = confirmationsCollection(c).FindOne(ctx, query, opts).Decode(&result); err != nil && err != mongo.ErrNoDocuments {
		return result, err
	}

	return result, nil
}

// FindConfirmations - find and return existing confirmations
func (c *MongoStoreClient) FindConfirmations(ctx context.Context, confirmation *models.Confirmation, statuses ...models.Status) (results []*models.Confirmation, err error) {
	var query bson.M = bson.M{}

	if confirmation.Email != "" {
		// case insensitive match
		// TODO: should use an index with collation, not a case-insensitive regex, since that can't use an index
		// However, we need MongoDB 3.2 to do this. See https://tidepool.atlassian.net/browse/BACK-1133
		// Collated index on type+email
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
	if confirmation.ClinicId != "" {
		query["clinicId"] = confirmation.ClinicId
	}

	if len(statuses) > 0 {
		query["status"] = bson.M{"$in": statuses}
	}

	opts := options.Find().SetSort(bson.D{{Key: "created", Value: -1}})
	cursor, err := confirmationsCollection(c).Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	if err = cursor.All(ctx, &results); err != nil {
		return results, err
	}
	return results, nil
}

// RemoveConfirmation - Remove a confirmation from the database
func (c *MongoStoreClient) RemoveConfirmation(ctx context.Context, confirmation *models.Confirmation) error {
	result := confirmationsCollection(c).FindOneAndDelete(ctx, bson.M{"_id": confirmation.Key})
	if result.Err() != mongo.ErrNoDocuments {
		return result.Err()
	}
	return nil
}

func (c *MongoStoreClient) RemoveConfirmationsForUser(ctx context.Context, userId string) error {
	selector := bson.M{
		"$or": []bson.M{
			{
				// Only delete clinic confirmations when the user is the receiver of the invite
				"$and": []bson.M{
					{"userId": userId},
					{"clinicId": bson.M{"$exists": true}},
				},
			},
			{
				// Delete non-clinic confirmation if the user is the sender or the receiver of the invite
				"$and": []bson.M{
					{"clinicId": nil},
					{"$or": []bson.M{
						{"userId": userId},
						{"creatorId": userId},
					}},
				},
			},
		},
	}
	_, err := confirmationsCollection(c).DeleteMany(ctx, selector)
	return err
}
