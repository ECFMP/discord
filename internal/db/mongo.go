package db

import (
	"context"
	pb "ecfmp/discord/proto"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DiscordMessage struct {
	Id              string `bson:"_id,omitempty"`
	ClientRequestId string `bson:"client_request_id"`
	Content         string `bson:"content"`
}

type Mongo struct {
	Client *mongo.Client
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

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Errorf("Failed to ping mongo: %v", err)
		return nil, err
	}

	return &Mongo{
		Client: client,
	}, nil
}

/**
 * Write a discord message to the database
 */
func (m *Mongo) WriteDiscordMessage(clientRequestId string, message *pb.DiscordMessage) (string, error) {
	collection := m.Client.Database("ecfmp").Collection("discord_messages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	record := DiscordMessage{
		ClientRequestId: clientRequestId,
		Content:         message.Content,
	}
	res, err := collection.InsertOne(ctx, record)
	if err != nil {
		return "", err
	}

	return res.InsertedID.(primitive.ObjectID).Hex(), nil
}

func (m *Mongo) GetDiscordMessageById(id string) (*DiscordMessage, error) {
	collection := m.Client.Database("ecfmp").Collection("discord_messages")
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
	collection := m.Client.Database("ecfmp").Collection("discord_messages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result DiscordMessage
	err := collection.FindOne(ctx, bson.M{"client_request_id": clientRequestId}).Decode(&result)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}

	return &result, nil
}

func (m *Mongo) Disconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.Client.Disconnect(ctx)
}
