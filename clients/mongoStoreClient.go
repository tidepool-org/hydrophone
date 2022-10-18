package clients

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"

	goComMgo "github.com/mdblp/go-db/mongo"
	"github.com/mdblp/hydrophone/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	confirmationsCollection = "confirmations"
)

// Client struct
type Client struct {
	*goComMgo.StoreClient
}

// NewStore creates a new Client
func NewStore(config *goComMgo.Config, logger *log.Logger) (*Client, error) {
	client := Client{}
	store, err := goComMgo.NewStoreClient(config, logger)
	client.StoreClient = store
	return &client, err
}

//warpper function for consistent access to the collection
func mgoConfirmationsCollection(c *Client) *mongo.Collection {
	return c.Collection(confirmationsCollection)
}

// UpsertConfirmation creates or updates a confirmation
func (c *Client) UpsertConfirmation(ctx context.Context, confirmation *models.Confirmation) error {
	options := options.Update().SetUpsert(true)
	update := bson.D{{"$set", confirmation}}
	_, err := mgoConfirmationsCollection(c).UpdateOne(ctx, bson.M{"_id": confirmation.Key}, update, options)
	return err
}

// FindConfirmation returns latest created confirmation matching filter passed as parameter
func (c *Client) FindConfirmation(ctx context.Context, confirmation *models.Confirmation) (result *models.Confirmation, err error) {

	var query bson.M = bson.M{}

	if confirmation.Email != "" {
		regexFilter := primitive.Regex{Pattern: fmt.Sprintf("^%s$", regexp.QuoteMeta(confirmation.Email)), Options: "i"}
		query["email"] = bson.M{"$regex": regexFilter}
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
	if confirmation.ShortKey != "" {
		query["shortKey"] = confirmation.ShortKey
	}
	if confirmation.Team != nil && confirmation.Team.ID != "" {
		query["teamId"] = confirmation.Team.ID
	}
	opts := options.FindOne()
	opts.SetSort(bson.D{primitive.E{Key: "created", Value: -1}})
	if err = mgoConfirmationsCollection(c).FindOne(ctx, query, opts).Decode(&result); err != nil && err != mongo.ErrNoDocuments {
		log.Printf("FindConfirmation: something bad happened [%v]", err)
		return result, err
	}

	return result, nil
}

// FindConfirmations returns all created confirmations matching filter passed as parameter
func (c *Client) FindConfirmations(ctx context.Context, confirmation *models.Confirmation, statuses []models.Status, types []models.Type) (results []*models.Confirmation, err error) {

	var query bson.M = bson.M{}

	if confirmation.Email != "" {
		regexFilter := primitive.Regex{Pattern: fmt.Sprintf("^%s$", regexp.QuoteMeta(confirmation.Email)), Options: "i"}
		query["email"] = bson.M{"$regex": regexFilter}
	}
	if confirmation.Key != "" {
		query["_id"] = confirmation.Key
	}
	if len(types) > 0 {
		query["type"] = bson.M{"$in": types}
	} else {
		if string(confirmation.Type) != "" {
			query["type"] = confirmation.Type
		}
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
	if confirmation.ShortKey != "" {
		query["shortKey"] = confirmation.ShortKey
	}
	if confirmation.Team != nil && confirmation.Team.ID != "" {
		query["teamId"] = confirmation.Team.ID
	}

	opts := options.Find()
	opts.SetSort(bson.D{primitive.E{Key: "created", Value: -1}})
	cursor, err := mgoConfirmationsCollection(c).Find(ctx, query, opts)
	defer cursor.Close(ctx)
	if err != nil {
		log.Printf("FindConfirmation: something bad happened [%v]", err)
		return results, err
	}
	err = cursor.All(ctx, &results)
	return results, err
}

// RemoveConfirmation deletes confirmation based on key (_id)
func (c *Client) RemoveConfirmation(ctx context.Context, confirmation *models.Confirmation) error {

	if _, err := mgoConfirmationsCollection(c).DeleteOne(ctx, bson.M{"_id": confirmation.Key}); err != nil {
		return err
	}
	return nil
}

func (c *Client) CountLatestConfirmations(ctx context.Context, confirmation models.Confirmation, createdSince time.Time) (int64, error) {
	if confirmation.CreatorId == "" && confirmation.Email == "" && confirmation.UserId == "" {
		return 0, errors.New("CreatorId or UserId or Email is missing")
	}
	var query bson.M = bson.M{}
	if confirmation.CreatorId != "" {
		query["creatorId"] = confirmation.CreatorId
	}
	if confirmation.Email != "" {
		query["email"] = confirmation.Email
	}
	if confirmation.UserId != "" {
		query["userId"] = confirmation.UserId
	}
	if confirmation.Type != "" {
		query["type"] = confirmation.Type
	}
	query["created"] = bson.M{"$gte": createdSince}
	count, err := mgoConfirmationsCollection(c).CountDocuments(ctx, query)
	return count, err
}
