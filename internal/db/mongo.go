package db

import (
	"context"
	pb "ecfmp/discord/proto/discord/gen/pb-go/ecfmp.vatsim.net/grpc/discord"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Mongo struct {
	Client   *mongo.Client
	database string
}

/**
 * Create a new mongo connection
 */
func NewMongo() (*Mongo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	auth := options.Credential{
		Username: os.Getenv("MONGO_USERNAME"),
		Password: os.Getenv("MONGO_PASSWORD"),
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_HOST")).SetAuth(auth).SetMaxPoolSize(10).SetMaxConnIdleTime(5*time.Second))
	if err != nil {
		log.Errorf("Failed to connect to mongo: %v", err)
		return nil, err
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

	// Create an index on a ttl field that will delete messages after 1 week
	_, indexErr = collection.Indexes().CreateOne(
		context.Background(),
		mongo.IndexModel{
			Keys: bson.M{
				"created_at": 1,
			},
			Options: options.Index().SetExpireAfterSeconds(604800).SetName("created_at"),
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
		Content:         message.Content,
		Embeds:          DiscordEmbedToMongo(&message.Embeds),
		CreatedAt:       time.Now(),
	}
	record := DiscordMessage{
		Channel:   message.Channel,
		Versions:  []DiscordMessageVersion{version},
		CreatedAt: time.Now(),
	}
	res, err := collection.InsertOne(ctx, record)
	if err != nil {
		return "", err
	}

	return res.InsertedID.(primitive.ObjectID).Hex(), nil
}

/**
 * Publish a discord message to the database
 */
func (m *Mongo) PublishMessageVersion(clientRequestId string, message *pb.UpdateRequest) error {
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
		Content:         message.Content,
		Embeds:          DiscordEmbedToMongo(&message.Embeds),
		CreatedAt:       time.Now(),
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

/**
 * Update the message with the discord id and the last publish request id, so we avoid publishing the same message twice.
 * Called when the message is published to discord for the first time.
 */
func (m *Mongo) UpdateMessageWithDiscordIdAndLastPublishRequest(id string, discordId string, requestId string) error {
	collection := m.Client.Database(m.database).Collection("discord_messages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objectId, idErr := primitive.ObjectIDFromHex(id)
	if idErr != nil {
		return idErr
	}

	// Update the message
	var updated DiscordMessage
	updateErr := collection.FindOneAndUpdate(ctx, bson.M{"_id": objectId}, bson.M{"$set": bson.M{"discord_id": discordId, "last_client_request_published": requestId}}).Decode(&updated)
	if updateErr != nil && updateErr == mongo.ErrNoDocuments {
		return fmt.Errorf("message not found")
	}

	if updateErr != nil {
		return updateErr
	}

	return nil
}

/**
 * Update the message with the last publish request id, so we avoid publishing the same message twice.
 * Called when the message is published to discord for subsequent times.
 */
func (m *Mongo) UpdateMessageWithLastPublishRequest(id string, requestId string) error {
	collection := m.Client.Database(m.database).Collection("discord_messages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objectId, idErr := primitive.ObjectIDFromHex(id)
	if idErr != nil {
		return idErr
	}

	// Update the message
	var updated DiscordMessage
	updateErr := collection.FindOneAndUpdate(ctx, bson.M{"_id": objectId}, bson.M{"$set": bson.M{"last_client_request_published": requestId}}).Decode(&updated)
	if updateErr != nil && updateErr == mongo.ErrNoDocuments {
		return fmt.Errorf("message not found")
	}

	if updateErr != nil {
		return updateErr
	}

	return nil
}

/**
 * Gets a discord message is present by id
 * Should handle the case where the message is not present without erroring
 */
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

/**
 * Disconnects from the mongo database.
 */
func (m *Mongo) Disconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.Client.Disconnect(ctx)
}
