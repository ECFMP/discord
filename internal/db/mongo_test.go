package db_test

import (
	"context"
	db "ecfmp/discord/internal/db"
	pb "ecfmp/discord/proto/discord"
	"testing"

	"github.com/stretchr/testify/assert"
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
	id, err := mongo.WriteDiscordMessage("1", &pb.DiscordMessage{Content: "Hello World!"})

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
	messageId, _ := mongo.WriteDiscordMessage("1", &pb.DiscordMessage{Content: "Hello World!"})

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
	mongo.WriteDiscordMessage("1", &pb.DiscordMessage{Content: "Hello World!"})

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
	messageId, _ := mongo.WriteDiscordMessage("1", &pb.DiscordMessage{Content: "Hello World!"})

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
	mongo.WriteDiscordMessage("1", &pb.DiscordMessage{Content: "Hello World!"})

	// Then
	message, requestErr := mongo.GetDiscordMessageById("65106dab41199f298668474f")
	assert.Nil(t, requestErr)
	assert.Nil(t, message)
}
