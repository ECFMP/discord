package db_test

import (
	"context"
	db "ecfmp/discord/internal/db"
	pb "ecfmp/discord/proto/discord"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func SetupTest(t *testing.T) func(tb testing.TB) {

	mongo, err := db.NewMongo()
	if err != nil {
		t.Errorf("Failed to connect to mongo: %v", err)
	}

	mongo.Client.Database("ecfmp_test").Collection("discord_messages").Drop(context.Background())

	return func(tb testing.TB) {
		mongo.Client.Disconnect(context.Background())
	}
}

func Test_ItWritesADiscordMessage(t *testing.T) {
	teardown := SetupTest(t)
	defer teardown(t)

	// Given
	mongo, err := db.NewMongo()
	if err != nil {
		t.Errorf("Failed to connect to mongo: %v", err)
	}

	// When
	id, err := mongo.WriteDiscordMessage("1", &pb.CreateRequest{Content: "Hello World!"})

	// Then
	assert.Nil(t, err)
	assert.NotEmpty(t, id)

	// Check the database
	var result db.DiscordMessage
	err = mongo.Client.Database("ecfmp_test").Collection("discord_messages").FindOne(context.Background(), map[string]string{"versions.client_request_id": "1"}).Decode(&result)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(result.Versions))
	assert.Equal(t, "1", result.Versions[0].ClientRequestId)
	assert.Equal(t, "Hello World!", result.Versions[0].Content)
}

func Test_ItGetsDiscordMessageByClientRequestId(t *testing.T) {
	teardown := SetupTest(t)
	defer teardown(t)

	// Given
	mongo, err := db.NewMongo()
	if err != nil {
		t.Errorf("Failed to connect to mongo: %v", err)
	}

	// When
	messageId, _ := mongo.WriteDiscordMessage("1", &pb.CreateRequest{Content: "Hello World!"})

	// Then
	message, requestErr := mongo.GetDiscordMessageByClientRequestId("1")
	assert.Nil(t, requestErr)
	assert.Equal(t, messageId, message.Id)
	assert.Equal(t, 1, len(message.Versions))
	assert.Equal(t, "1", message.Versions[0].ClientRequestId)
	assert.Equal(t, "Hello World!", message.Versions[0].Content)
}

func Test_ItDoesntFindMessageByClientRequestId(t *testing.T) {
	teardown := SetupTest(t)
	defer teardown(t)

	// Given
	mongo, err := db.NewMongo()
	if err != nil {
		t.Errorf("Failed to connect to mongo: %v", err)
	}

	// When
	mongo.WriteDiscordMessage("1", &pb.CreateRequest{Content: "Hello World!"})

	// Then
	message, requestErr := mongo.GetDiscordMessageByClientRequestId("2")
	assert.Nil(t, requestErr)
	assert.Nil(t, message)
}

func Test_ItGetsDiscordMessageById(t *testing.T) {
	teardown := SetupTest(t)
	defer teardown(t)

	// Given
	mongo, err := db.NewMongo()
	if err != nil {
		t.Errorf("Failed to connect to mongo: %v", err)
	}

	// When
	messageId, _ := mongo.WriteDiscordMessage("1", &pb.CreateRequest{Content: "Hello World!"})

	// Then
	message, requestErr := mongo.GetDiscordMessageById(messageId)
	assert.Nil(t, requestErr)
	assert.Equal(t, messageId, message.Id)
}

func Test_ItDoesntFindMessageById(t *testing.T) {
	teardown := SetupTest(t)
	defer teardown(t)

	// Given
	mongo, err := db.NewMongo()
	if err != nil {
		t.Errorf("Failed to connect to mongo: %v", err)
	}

	// When
	mongo.WriteDiscordMessage("1", &pb.CreateRequest{Content: "Hello World!"})

	// Then
	message, requestErr := mongo.GetDiscordMessageById("65106dab41199f298668474f")
	assert.Nil(t, requestErr)
	assert.Nil(t, message)
}

func Test_ItReturnsErrorFindingBadId(t *testing.T) {
	teardown := SetupTest(t)
	defer teardown(t)

	// Given
	mongo, err := db.NewMongo()
	if err != nil {
		t.Errorf("Failed to connect to mongo: %v", err)
	}

	// When
	mongo.WriteDiscordMessage("1", &pb.CreateRequest{Content: "Hello World!"})

	// Then
	_, requestErr := mongo.GetDiscordMessageById("abc")
	assert.Equal(t, "the provided hex string is not a valid ObjectID", requestErr.Error())
}

func Test_ItUpdatesMessageById(t *testing.T) {
	teardown := SetupTest(t)
	defer teardown(t)

	// Given
	mongo, err := db.NewMongo()
	if err != nil {
		t.Errorf("Failed to connect to mongo: %v", err)
	}

	// When
	id, _ := mongo.WriteDiscordMessage("1", &pb.CreateRequest{Content: "Hello World!"})
	publishErr := mongo.PublishMessageVersion("another-request-id", &pb.UpdateRequest{Id: id, Content: "Hello Go!"})
	assert.Nil(t, publishErr)

	// Then
	objectId, _ := primitive.ObjectIDFromHex(id)
	var result db.DiscordMessage
	err = mongo.Client.Database("ecfmp_test").Collection("discord_messages").FindOne(context.Background(), bson.M{"_id": objectId}).Decode(&result)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(result.Versions))
	assert.Equal(t, "1", result.Versions[0].ClientRequestId)
	assert.Equal(t, "Hello World!", result.Versions[0].Content)
	assert.Equal(t, "another-request-id", result.Versions[1].ClientRequestId)
	assert.Equal(t, "Hello Go!", result.Versions[1].Content)
}

func Test_ItReturnsErrorUpdatingNonExistentMessage(t *testing.T) {
	teardown := SetupTest(t)
	defer teardown(t)

	// Given
	mongo, err := db.NewMongo()
	if err != nil {
		t.Errorf("Failed to connect to mongo: %v", err)
	}

	// When
	mongo.WriteDiscordMessage("1", &pb.CreateRequest{Content: "Hello World!"})
	publishErr := mongo.PublishMessageVersion("another-request-id", &pb.UpdateRequest{Id: "65106dab41199f298668474f", Content: "Hello Go!"})
	assert.Equal(t, "message not found", publishErr.Error())
}

func Test_ItReturnsErrorUpdatingBadId(t *testing.T) {
	teardown := SetupTest(t)
	defer teardown(t)

	// Given
	mongo, err := db.NewMongo()
	if err != nil {
		t.Errorf("Failed to connect to mongo: %v", err)
	}

	// When
	mongo.WriteDiscordMessage("1", &pb.CreateRequest{Content: "Hello World!"})
	publishErr := mongo.PublishMessageVersion("another-request-id", &pb.UpdateRequest{Id: "abc", Content: "Hello Go!"})
	assert.Equal(t, "the provided hex string is not a valid ObjectID", publishErr.Error())
}

func Test_ItUpdatesAMessageWithDiscordIdAndPublishedId(t *testing.T) {
	teardown := SetupTest(t)
	defer teardown(t)

	// Given
	mongo, mongoErr := db.NewMongo()
	if mongoErr != nil {
		t.Errorf("Failed to connect to mongo: %v", mongoErr)
	}

	// When
	id, _ := mongo.WriteDiscordMessage("1", &pb.CreateRequest{Content: "Hello World!"})
	publishErr := mongo.PublishMessageVersion("another-request-id", &pb.UpdateRequest{Id: id, Content: "Hello Go!"})
	assert.Nil(t, publishErr)
	updateErr := mongo.UpdateMessageWithDiscordIdAndLastPublishRequest(id, "discord-id", "another-request-id")
	assert.Nil(t, updateErr)

	// Then
	objectId, _ := primitive.ObjectIDFromHex(id)
	var result db.DiscordMessage
	err := mongo.Client.Database("ecfmp_test").Collection("discord_messages").FindOne(context.Background(), bson.M{"_id": objectId}).Decode(&result)
	assert.Nil(t, err)
	assert.Equal(t, "discord-id", result.DiscordId)
	assert.Equal(t, "another-request-id", result.LastClientRequestPublished)
}

func Test_ItUpdatesAMessageWithPublishedId(t *testing.T) {
	teardown := SetupTest(t)
	defer teardown(t)

	// Given
	mongo, mongoErr := db.NewMongo()
	if mongoErr != nil {
		t.Errorf("Failed to connect to mongo: %v", mongoErr)
	}

	// When
	id, _ := mongo.WriteDiscordMessage("1", &pb.CreateRequest{Content: "Hello World!"})
	publishErr := mongo.PublishMessageVersion("another-request-id", &pb.UpdateRequest{Id: id, Content: "Hello Go!"})
	assert.Nil(t, publishErr)
	updateErr := mongo.UpdateMessageWithLastPublishRequest(id, "another-request-id")
	assert.Nil(t, updateErr)

	// Then
	objectId, _ := primitive.ObjectIDFromHex(id)
	var result db.DiscordMessage
	err := mongo.Client.Database("ecfmp_test").Collection("discord_messages").FindOne(context.Background(), bson.M{"_id": objectId}).Decode(&result)
	assert.Nil(t, err)
	assert.Equal(t, "another-request-id", result.LastClientRequestPublished)
}
