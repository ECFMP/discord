package discord_test

import (
	"context"
	db "ecfmp/discord/internal/db"
	discord "ecfmp/discord/internal/discord"
	pb "ecfmp/discord/proto/discord/gen/pb-go/ecfmp.vatsim.net/grpc/discord"
	"os"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type MockDiscord struct {
	callCount     int
	callChannel   string
	callVersion   db.DiscordMessageVersion
	callDiscordId string
}

// publishMessage implements discord.Discord.
func (d *MockDiscord) PublishMessage(channelId string, version *db.DiscordMessageVersion) (string, error) {
	d.callCount++
	d.callChannel = channelId
	d.callVersion = *version
	return "123", nil
}

// updateMessage implements discord.Discord.
func (d *MockDiscord) UpdateMessage(channelId string, version *db.DiscordMessageVersion, discordId string) error {
	d.callCount++
	d.callChannel = channelId
	d.callVersion = *version
	d.callDiscordId = discordId
	return nil
}

type TestMongo struct {
	client   *db.Mongo
	tearDown func()
}

func SetupTest(t *testing.T) (*TestMongo, *MockDiscord, *discord.DiscordScheduler) {
	// Turn off logging except for fatals
	log.SetLevel(log.FatalLevel)

	mongo, err := db.NewMongo()
	if err != nil {
		t.Errorf("Failed to connect to mongo: %v", err)
	}

	mongo.Client.Database(os.Getenv("MONGO_DB")).Collection("discord_messages").Drop(context.Background())

	mockDiscord := &MockDiscord{}
	scheduler := discord.NewDiscordScheduler(mongo, mockDiscord)

	return &TestMongo{
		client: mongo,
		tearDown: func() {
			mongo.Client.Disconnect(context.Background())
		},
	}, mockDiscord, scheduler
}

func Test_ItPublishesNewMessages(t *testing.T) {
	testMongo, mockDiscord, scheduler := SetupTest(t)
	defer testMongo.tearDown()

	// Write to mongo (as this is done before now)
	mongoId, err := testMongo.client.WriteDiscordMessage("some-client-request-id", &pb.CreateRequest{Channel: "channel", Content: "Hello World"})
	if err != nil {
		t.Errorf("Failed to write to mongo: %v", err)
	}

	// Run the scheduler
	scheduler.ScheduleMessage(mongoId)

	// Wait for the scheduler to finish
	scheduler.GoRoutineWaitGroup.Wait()

	// Assert that the message was published to discord
	assert.Equal(t, 1, mockDiscord.callCount)
	assert.Equal(t, "Hello World", mockDiscord.callVersion.Content)
	assert.Equal(t, "channel", mockDiscord.callChannel)

	// Check that the message was written to mongo
	mongoMessage, mongoErr := testMongo.client.GetDiscordMessageById(mongoId)
	if mongoErr != nil {
		t.Errorf("Failed to get message from mongo: %v", mongoErr)
	}

	assert.Equal(t, "123", mongoMessage.DiscordId)
	assert.Equal(t, "some-client-request-id", mongoMessage.LastClientRequestPublished)
}

func Test_ItUpdatesMessagesFromVersions(t *testing.T) {
	testMongo, mockDiscord, scheduler := SetupTest(t)
	defer testMongo.tearDown()

	// Write to mongo (as this is done before now)
	mongoId, err := testMongo.client.WriteDiscordMessage("some-client-request-id", &pb.CreateRequest{Channel: "channel", Content: "Hello World"})
	if err != nil {
		t.Errorf("Failed to write to mongo: %v", err)
	}

	// Update the message to have a discord id
	testMongo.client.UpdateMessageWithDiscordIdAndLastPublishRequest(mongoId, "123", "some-other-client-request-id")

	// Run the scheduler
	scheduler.ScheduleMessage(mongoId)

	// Wait for the scheduler to finish
	scheduler.GoRoutineWaitGroup.Wait()

	// Assert that the message was published to discord
	assert.Equal(t, 1, mockDiscord.callCount)
	assert.Equal(t, "Hello World", mockDiscord.callVersion.Content)
	assert.Equal(t, "123", mockDiscord.callDiscordId)
	assert.Equal(t, "channel", mockDiscord.callChannel)

	// Check that the message was written to mongo
	mongoMessage, mongoErr := testMongo.client.GetDiscordMessageById(mongoId)
	if mongoErr != nil {
		t.Errorf("Failed to get message from mongo: %v", mongoErr)
	}

	assert.Equal(t, "some-client-request-id", mongoMessage.LastClientRequestPublished)
}

func Test_ItReturnsReadyStatus(t *testing.T) {
	testMongo, _, scheduler := SetupTest(t)
	defer testMongo.tearDown()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		if ctx.Err() != nil {
			t.Errorf("Timed out waiting for scheduler to be ready")
			break
		}

		if scheduler.Ready() {
			break
		}
	}
}
