package db

import (
	"context"
	pb "ecfmp/discord/proto/discord"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DiscordMessageVersion struct {
	ClientRequestId string `bson:"client_request_id"`
	Content         string `bson:"content"`
}

type DiscordMessage struct {
	Id              string `bson:"_id,omitempty"`
	LastClientRequestPublished string `bson:"last_client_request_published"`
	Versions []DiscordMessageVersion `bson:"versions"`
}

// THOUGHTS:
// When a message is sent for the first time, it will have a client request id, we also generate another id
// On updates, we publish the latest for that id
// We probably also want a log of all the client requests that have been published
// So we have a structure of id, last_client_request_published, versions (client_request_id, content, timestamp)
// On insert/update, we check that we dont have duplicate client request id, if we do, we don't do anything
// Once we've done the update, we put the client request id onto a queue for publishing
// At publish time, we publish the latest version for each id and update the last_client_request_published
// If there's a race of two updates for the same id, we'll just publish the latest one (and ignore the next job when it comes)

type Mongo struct {
	Client   *mongo.Client
	database string
}

func NewMongo() (*Mongo, error) {
	mongoUri := fmt.Sprintf(
		"mongodb://%s:%s@%s:%s",
		os.Getenv("MONGO_USERNAME"),
		os.Getenv("MONGO_PASSWORD"),
		os.Getenv("MONGO_HOST"),
		os.Getenv("MONGO_PORT"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoUri))
	if err != nil {
		log.Errorf("Failed to connect to mongo: %v", err)
		return nil, err
	}

	// Ping mongo
	pingErr := client.Ping(ctx, nil)
	if pingErr != nil {
		log.Errorf("Failed to ping mongo: %v", pingErr)
		return nil, pingErr
	}

	// Create necessary indexes in mongo
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("discord_messages")
	_, indexErr := collection.Indexes().CreateOne(
		context.Background(),
		mongo.IndexModel{
			Keys: bson.M{
				"versions.client_request_id": 1,
			},
			Options: options.Index().SetUnique(true).SetName("versions_client_request_id"),
		},
	)

	if indexErr != nil {
		log.Errorf("Failed to create index: %v", indexErr)
		return nil, indexErr
	}

	return &Mongo{
		Client:   client,
		database: os.Getenv("MONGO_DB"),
	}, nil
}

/**
 * Write a discord message to the database
 */
func (m *Mongo) WriteDiscordMessage(clientRequestId string, message *pb.CreateRequest) (string, error) {
	collection := m.Client.Database(m.database).Collection("discord_messages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	version := DiscordMessageVersion{
		ClientRequestId: clientRequestId,
		Content: message.Content,
	}
	record := DiscordMessage{
		Versions: []DiscordMessageVersion{version},
	}
	res, err := collection.InsertOne(ctx, record)
	if err != nil {
		return "", err
	}

	return res.InsertedID.(primitive.ObjectID).Hex(), nil
}

func (m *Mongo) PublishMessageVersion(clientRequestId string, message *pb.UpdateRequest) (error) {
	collection := m.Client.Database(m.database).Collection("discord_messages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objectId, idErr := primitive.ObjectIDFromHex(message.Id)
	if idErr != nil {
		return idErr
	}

	// Update the message
	version := DiscordMessageVersion{
		ClientRequestId: clientRequestId,
		Content: message.Content,
	}
	updateCount, err := collection.UpdateByID(ctx, objectId, bson.M{"$push": bson.M{"versions": version}})
	if err != nil {
		return err
	}

	if updateCount.ModifiedCount != 1 {
		return fmt.Errorf("message not found")
	}

	return nil
}

func (m *Mongo) GetDiscordMessageById(id string) (*DiscordMessage, error) {
	collection := m.Client.Database(m.database).Collection("discord_messages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objectId, idErr := primitive.ObjectIDFromHex(id)
	if idErr != nil {
		return nil, idErr
	}

	var result DiscordMessage
	err := collection.FindOne(ctx, bson.M{"_id": objectId}).Decode(&result)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}

	return &result, nil
}

/**
 * Gets a discord message is present by client request id
 * Should handle the case where the message is not present without erroring
 */
func (m *Mongo) GetDiscordMessageByClientRequestId(clientRequestId string) (*DiscordMessage, error) {
	collection := m.Client.Database(m.database).Collection("discord_messages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result DiscordMessage
	err := collection.FindOne(ctx, bson.M{"versions.client_request_id": clientRequestId}).Decode(&result)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}

	return &result, nil
}

func (m *Mongo) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.Client.Ping(ctx, nil)
}

func (m *Mongo) Disconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.Client.Disconnect(ctx)
}
