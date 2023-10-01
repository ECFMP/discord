package db_test

import (
	"context"
	db "ecfmp/discord/internal/db"
	pb "ecfmp/discord/proto/discord"
	"testing"
	"time"

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
	id, err := mongo.WriteDiscordMessage(
		"1",
		&pb.CreateRequest{
			Content: "Hello World!",
			Embeds: []*pb.DiscordEmbeds{
				{
					Title:       "Hello World!",
					Description: "This is a test",
					Url:         "https://example.com",
					Color:       123456,
					Fields: []*pb.DiscordEmbedsFields{
						{
							Name:   "Field 1",
							Value:  "Value 1",
							Inline: true,
						},
						{
							Name:   "Field 2",
							Value:  "Value 2",
							Inline: false,
						},
					},
				},
			},
		})

	// Then
	assert.Nil(t, err)
	if err != nil {
		t.Errorf("BIG ERR: %v", err)
	}
	assert.NotEmpty(t, id)

	// Check the database
	var result db.DiscordMessage
	err = mongo.Client.Database("ecfmp_test").Collection("discord_messages").FindOne(context.Background(), map[string]string{"versions.client_request_id": "1"}).Decode(&result)
	assert.Nil(t, err)
	assert.Equal(t, id, result.Id)
	assert.GreaterOrEqual(t, result.CreatedAt.Unix(), time.Now().Unix()-5)
	assert.LessOrEqual(t, result.CreatedAt.Unix(), time.Now().Unix())
	assert.Equal(t, 1, len(result.Versions))
	assert.Equal(t, "1", result.Versions[0].ClientRequestId)
	assert.GreaterOrEqual(t, result.Versions[0].CreatedAt.Unix(), time.Now().Unix()-5)
	assert.LessOrEqual(t, result.Versions[0].CreatedAt.Unix(), time.Now().Unix())
	assert.Equal(t, "Hello World!", result.Versions[0].Content)
	assert.Equal(t, 1, len(result.Versions[0].Embeds))
	assert.Equal(t, "Hello World!", result.Versions[0].Embeds[0].Title)
	assert.Equal(t, "This is a test", result.Versions[0].Embeds[0].Description)
	assert.Equal(t, "https://example.com", result.Versions[0].Embeds[0].Url)
	assert.Equal(t, int32(123456), result.Versions[0].Embeds[0].Color)
	assert.Equal(t, 2, len(result.Versions[0].Embeds[0].Fields))
	assert.Equal(t, "Field 1", result.Versions[0].Embeds[0].Fields[0].Name)
	assert.Equal(t, "Value 1", result.Versions[0].Embeds[0].Fields[0].Value)
	assert.Equal(t, true, result.Versions[0].Embeds[0].Fields[0].Inline)
	assert.Equal(t, "Field 2", result.Versions[0].Embeds[0].Fields[1].Name)
	assert.Equal(t, "Value 2", result.Versions[0].Embeds[0].Fields[1].Value)
	assert.Equal(t, false, result.Versions[0].Embeds[0].Fields[1].Inline)
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
	publishErr := mongo.PublishMessageVersion(
		"another-request-id",
		&pb.UpdateRequest{
			Id:      id,
			Content: "Hello Go!",
			Embeds: []*pb.DiscordEmbeds{
				{
					Title:       "Hello World!",
					Description: "This is a test",
					Url:         "https://example.com",
					Color:       123456,
					Fields: []*pb.DiscordEmbedsFields{
						{
							Name:   "Field 1",
							Value:  "Value 1",
							Inline: true,
						},
						{
							Name:   "Field 2",
							Value:  "Value 2",
							Inline: false,
						},
					},
				},
			},
		},
	)
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
	assert.GreaterOrEqual(t, result.Versions[1].CreatedAt.Unix(), time.Now().Unix()-5)
	assert.LessOrEqual(t, result.Versions[1].CreatedAt.Unix(), time.Now().Unix())
	assert.Equal(t, 1, len(result.Versions[1].Embeds))
	assert.Equal(t, "Hello World!", result.Versions[1].Embeds[0].Title)
	assert.Equal(t, "This is a test", result.Versions[1].Embeds[0].Description)
	assert.Equal(t, "https://example.com", result.Versions[1].Embeds[0].Url)
	assert.Equal(t, int32(123456), result.Versions[1].Embeds[0].Color)
	assert.Equal(t, 2, len(result.Versions[1].Embeds[0].Fields))
	assert.Equal(t, "Field 1", result.Versions[1].Embeds[0].Fields[0].Name)
	assert.Equal(t, "Value 1", result.Versions[1].Embeds[0].Fields[0].Value)
	assert.Equal(t, true, result.Versions[1].Embeds[0].Fields[0].Inline)
	assert.Equal(t, "Field 2", result.Versions[1].Embeds[0].Fields[1].Name)
	assert.Equal(t, "Value 2", result.Versions[1].Embeds[0].Fields[1].Value)
	assert.Equal(t, false, result.Versions[1].Embeds[0].Fields[1].Inline)
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

func Test_ItMarshallsDiscordEmbedFieldToLibrarySend(t *testing.T) {
	// Given
	embedField := &db.DiscordEmbedField{
		Name:   "Field 1",
		Value:  "Value 1",
		Inline: true,
	}

	// When
	marshalled := embedField.MarshallToLibraryMessageSend()

	// Then
	assert.Equal(t, "Field 1", marshalled.Name)
	assert.Equal(t, "Value 1", marshalled.Value)
	assert.Equal(t, true, marshalled.Inline)
}

func Test_ItMarshallsDiscordEmbedToLibrarySend(t *testing.T) {
	// Given
	embed := &db.DiscordEmbed{
		Title:       "Hello World!",
		Description: "This is a test",
		Url:         "https://example.com",
		Color:       123456,
		Fields: []db.DiscordEmbedField{
			{
				Name:   "Field 1",
				Value:  "Value 1",
				Inline: true,
			},
			{
				Name:   "Field 2",
				Value:  "Value 2",
				Inline: false,
			},
		},
	}

	// When
	marshalled := embed.MarshallToLibraryMessageSend()

	// Then
	assert.Equal(t, "Hello World!", marshalled.Title)
	assert.Equal(t, "This is a test", marshalled.Description)
	assert.Equal(t, "https://example.com", marshalled.URL)
	assert.Equal(t, 123456, marshalled.Color)
	assert.Equal(t, 2, len(marshalled.Fields))
	assert.Equal(t, "Field 1", marshalled.Fields[0].Name)
	assert.Equal(t, "Value 1", marshalled.Fields[0].Value)
	assert.Equal(t, true, marshalled.Fields[0].Inline)
	assert.Equal(t, "Field 2", marshalled.Fields[1].Name)
	assert.Equal(t, "Value 2", marshalled.Fields[1].Value)
	assert.Equal(t, false, marshalled.Fields[1].Inline)
}

func Test_ItMarshallsDiscordEmbedToLibrarySendMinimalData(t *testing.T) {
	// Given
	embed := &db.DiscordEmbed{}

	// When
	marshalled := embed.MarshallToLibraryMessageSend()

	// Then
	assert.Equal(t, "", marshalled.Title)
	assert.Equal(t, "", marshalled.Description)
	assert.Equal(t, "", marshalled.URL)
	assert.Equal(t, 0, marshalled.Color)
	assert.Equal(t, 0, len(marshalled.Fields))
}

func Test_ItMarshallsVersionsToLibrarySend(t *testing.T) {
	// Given
	version := db.DiscordMessageVersion{
		ClientRequestId: "1",
		Content:         "Hello World!",
		Embeds: []db.DiscordEmbed{
			{
				Title:       "Hello World!",
				Description: "This is a test",
				Url:         "https://example.com",
				Color:       123456,
				Fields: []db.DiscordEmbedField{
					{
						Name:   "Field 1",
						Value:  "Value 1",
						Inline: true,
					},
					{
						Name:   "Field 2",
						Value:  "Value 2",
						Inline: false,
					},
				},
			},
		},
	}

	// When
	marshalled := version.MarshallToLibraryMessageSend()

	// Then
	assert.Equal(t, "Hello World!", marshalled.Content)
	assert.Equal(t, 1, len(marshalled.Embeds))
	assert.Equal(t, "Hello World!", marshalled.Embeds[0].Title)
	assert.Equal(t, "This is a test", marshalled.Embeds[0].Description)
	assert.Equal(t, 123456, marshalled.Embeds[0].Color)
	assert.Equal(t, 2, len(marshalled.Embeds[0].Fields))
	assert.Equal(t, "Field 1", marshalled.Embeds[0].Fields[0].Name)
	assert.Equal(t, "Value 1", marshalled.Embeds[0].Fields[0].Value)
	assert.Equal(t, true, marshalled.Embeds[0].Fields[0].Inline)
	assert.Equal(t, "Field 2", marshalled.Embeds[0].Fields[1].Name)
	assert.Equal(t, "Value 2", marshalled.Embeds[0].Fields[1].Value)
	assert.Equal(t, false, marshalled.Embeds[0].Fields[1].Inline)
}

func Test_ItMarshallsVersionsToLibrarySendNoEmbeds(t *testing.T) {
	// Given
	version := db.DiscordMessageVersion{
		ClientRequestId: "1",
		Content:         "Hello World!",
	}

	// When
	marshalled := version.MarshallToLibraryMessageSend()

	// Then
	assert.Equal(t, "Hello World!", marshalled.Content)
	assert.Equal(t, 0, len(marshalled.Embeds))
}

func Test_ItMarshallsVersionsToLibraryEdit(t *testing.T) {
	// Given
	version := db.DiscordMessageVersion{
		ClientRequestId: "1",
		Content:         "Hello World!",
		Embeds: []db.DiscordEmbed{
			{
				Title:       "Hello World!",
				Description: "This is a test",
				Url:         "https://example.com",
				Color:       123456,
				Fields: []db.DiscordEmbedField{
					{
						Name:   "Field 1",
						Value:  "Value 1",
						Inline: true,
					},
					{
						Name:   "Field 2",
						Value:  "Value 2",
						Inline: false,
					},
				},
			},
		},
	}

	// When
	marshalled := version.MarshallToLibraryMessageEdit("my-channel", "123")

	// Then
	assert.Equal(t, "my-channel", marshalled.Channel)
	assert.Equal(t, "123", marshalled.ID)
	assert.Equal(t, "Hello World!", *marshalled.Content)
	assert.Equal(t, 1, len(marshalled.Embeds))
	assert.Equal(t, "Hello World!", marshalled.Embeds[0].Title)
	assert.Equal(t, "This is a test", marshalled.Embeds[0].Description)
	assert.Equal(t, 123456, marshalled.Embeds[0].Color)
	assert.Equal(t, 2, len(marshalled.Embeds[0].Fields))
	assert.Equal(t, "Field 1", marshalled.Embeds[0].Fields[0].Name)
	assert.Equal(t, "Value 1", marshalled.Embeds[0].Fields[0].Value)
	assert.Equal(t, true, marshalled.Embeds[0].Fields[0].Inline)
	assert.Equal(t, "Field 2", marshalled.Embeds[0].Fields[1].Name)
	assert.Equal(t, "Value 2", marshalled.Embeds[0].Fields[1].Value)
	assert.Equal(t, false, marshalled.Embeds[0].Fields[1].Inline)
}

func Test_ItMarshallsVersionsToLibraryEditNoEmbeds(t *testing.T) {
	// Given
	version := db.DiscordMessageVersion{
		ClientRequestId: "1",
		Content:         "Hello World!",
	}

	// When
	marshalled := version.MarshallToLibraryMessageEdit("my-channel", "123")

	// Then
	assert.Equal(t, "my-channel", marshalled.Channel)
	assert.Equal(t, "Hello World!", *marshalled.Content)
	assert.Equal(t, 0, len(marshalled.Embeds))
}
