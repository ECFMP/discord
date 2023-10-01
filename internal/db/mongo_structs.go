package db

import (
	pb "ecfmp/discord/proto/discord"
	"time"

	discordgo "github.com/bwmarrin/discordgo"
)

/**
 * DiscordEmbedField is a struct that represents a Discord Embed Field.
 */
type DiscordEmbedField struct {
	Name   string `bson:"name"`
	Value  string `bson:"value"`
	Inline bool   `bson:"is_inline"`
}

/**
 * MarshallToLibraryMessageSend converts a DiscordEmbedField to a DiscordGo MessageEmbedField.
 */
func (d *DiscordEmbedField) MarshallToLibraryMessageSend() *discordgo.MessageEmbedField {
	return &discordgo.MessageEmbedField{
		Name:   d.Name,
		Value:  d.Value,
		Inline: d.Inline,
	}
}

/**
 * DiscordEmbedFieldsToMongo converts the DiscordEmbedFields from the protobuf to the mongo struct.
 */
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

/**
 * DiscordEmbed is a struct that represents a Discord Embed.
 */
type DiscordEmbed struct {
	Title       string              `bson:"title"`
	Description string              `bson:"description"`
	Url         string              `bson:"url"`
	Color       int32               `bson:"color"`
	Fields      []DiscordEmbedField `bson:"fields"`
}

/**
 * MarshallToLibraryMessageSend converts a DiscordEmbed to a DiscordGo MessageEmbed.
 */
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

/**
 * DiscordEmbedsToMongo converts the DiscordEmbeds from the protobuf to the mongo struct.
 */
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

/**
 * DiscordMessageVersion is a struct that represents a version of a Discord Message.
 */
type DiscordMessageVersion struct {
	ClientRequestId string         `bson:"client_request_id"`
	Content         string         `bson:"content"`
	Embeds          []DiscordEmbed `bson:"embeds"`
	CreatedAt       time.Time      `bson:"created_at"`
}

/**
 * MarshallToLibraryMessageSend converts a DiscordMessageVersion to a DiscordGo MessageSend for first
 * time publishing.
 */
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

/**
 * MarshallToLibraryMessageEdit converts a DiscordMessageVersion to a DiscordGo MessageEdit for updating
 * an existing message.
 */
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

/**
 * DiscordMessage is a struct that represents a Discord Message.
 */
type DiscordMessage struct {
	Id                         string                  `bson:"_id,omitempty"`
	DiscordId                  string                  `bson:"discord_id"`
	LastClientRequestPublished string                  `bson:"last_client_request_published"`
	Versions                   []DiscordMessageVersion `bson:"versions"`
	CreatedAt                  time.Time               `bson:"created_at"`
}
