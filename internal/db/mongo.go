package db

import (
	"context"
	pb "ecfmp/discord/proto/discord"
	"fmt"
	"os"
	"time"

	discordgo "github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DiscordEmbedField struct {
	Name   string `bson:"name"`
	Value  string `bson:"value"`
	Inline bool   `bson:"is_inline"`
}

func (d *DiscordEmbedField) MarshallToLibraryMessageSend() *discordgo.MessageEmbedField {
	return &discordgo.MessageEmbedField{
		Name:   d.Name,
		Value:  d.Value,
		Inline: d.Inline,
	}
}

type DiscordEmbed struct {
	Title       string              `bson:"title"`
	Description string              `bson:"description"`
	Url         string              `bson:"url"`
	Color       int32               `bson:"color"`
	Fields      []DiscordEmbedField `bson:"fields"`
}

func (d *DiscordEmbed) MarshallToLibraryMessageSend() *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{}

	if d.Title != "" {
		embed.Title = d.Title
	}

	if d.Description != "" {
		embed.Description = d.Description
	}

	if d.Url != "" {
		embed.URL = d.Url
	}

	if d.Color != 0 {
		embed.Color = int(d.Color)
	}

	if len(d.Fields) > 0 {
		embed.Fields = make([]*discordgo.MessageEmbedField, len(d.Fields))
		for i := range d.Fields {
			embed.Fields[i] = d.Fields[i].MarshallToLibraryMessageSend()
		}
	}

	return embed
}

func DiscordEmbedToMongo(embeds *[]*pb.DiscordEmbeds) []DiscordEmbed {
	result := make([]DiscordEmbed, len(*embeds))
	for i := range *embeds {
		embed := &(*embeds)[i]
		result[i] = DiscordEmbed{
			Title:       (*embed).Title,
			Description: (*embed).Description,
			Url:         (*embed).Url,
			Color:       (*embed).Color,
			Fields:      DiscordEmbedFieldsToMongo(&(*embed).Fields),
		}
	}
	return result
}

func DiscordEmbedFieldsToMongo(fields *[]*pb.DiscordEmbedsFields) []DiscordEmbedField {
	result := make([]DiscordEmbedField, len(*fields))
	for i := range *fields {
		field := &(*fields)[i]
		result[i] = DiscordEmbedField{
			Name:   (*field).Name,
			Value:  (*field).Value,
			Inline: (*field).Inline,
		}
	}
	return result
}

type DiscordMessageVersion struct {
	ClientRequestId string         `bson:"client_request_id"`
	Content         string         `bson:"content"`
	Embeds          []DiscordEmbed `bson:"embeds"`
	CreatedAt       time.Time      `bson:"created_at"`
}

func (d *DiscordMessageVersion) MarshallToLibraryMessageSend() *discordgo.MessageSend {
	// Create the message
	embeds := make([]*discordgo.MessageEmbed, len(d.Embeds))
	for i := range d.Embeds {
		embeds[i] = d.Embeds[i].MarshallToLibraryMessageSend()
	}

	return &discordgo.MessageSend{
		Content: d.Content,
		TTS:     false,
		Embeds:  embeds,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{
				discordgo.AllowedMentionTypeUsers,
				discordgo.AllowedMentionTypeRoles,
			},
		},
	}
}

func (d *DiscordMessageVersion) MarshallToLibraryMessageEdit(channel string, id string) *discordgo.MessageEdit {
	// Create the message
	embeds := make([]*discordgo.MessageEmbed, len(d.Embeds))
	for i := range d.Embeds {
		embeds[i] = d.Embeds[i].MarshallToLibraryMessageSend()
	}

	return &discordgo.MessageEdit{
		ID:      id,
		Channel: channel,
		Content: &d.Content,
		Embeds:  embeds,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{
				discordgo.AllowedMentionTypeUsers,
				discordgo.AllowedMentionTypeRoles,
			},
		},
	}
}

type DiscordMessage struct {
	Id                         string                  `bson:"_id,omitempty"`
	DiscordId                  string                  `bson:"discord_id"`
	LastClientRequestPublished string                  `bson:"last_client_request_published"`
	Versions                   []DiscordMessageVersion `bson:"versions"`
	CreatedAt                  time.Time               `bson:"created_at"`
}

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
		Versions:  []DiscordMessageVersion{version},
		CreatedAt: time.Now(),
	}
	res, err := collection.InsertOne(ctx, record)
	if err != nil {
		return "", err
	}

	return res.InsertedID.(primitive.ObjectID).Hex(), nil
}

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
