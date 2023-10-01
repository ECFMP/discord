package discord

import (
	db "ecfmp/discord/internal/db"
	"encoding/json"

	discordgo "github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

type Discord interface {
	PublishMessage(version *db.DiscordMessageVersion) (string, error)
	UpdateMessage(version *db.DiscordMessageVersion, discordId string) error
}

type DiscordPublisher struct {
	discord   *discordgo.Session
	channelId string
}

/**
 * Creates a new discord publisher.
 */
func NewDiscordPublisher(token string, channelId string) *DiscordPublisher {
	log.Infof("Creating discord publisher for channel %v", channelId)
	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("Failed to create discord session: %v", err)
	}

	return &DiscordPublisher{
		discord:   discord,
		channelId: channelId,
	}
}

/**
 * Publishes a message to discord.
 */
func (d *DiscordPublisher) PublishMessage(version *db.DiscordMessageVersion) (string, error) {
	someJson, _ := json.Marshal(version.MarshallToLibraryMessageSend())
	log.Infof("Publishing message to discord: %v", string(someJson[:]))
	message, err := d.discord.ChannelMessageSendComplex(d.channelId, version.MarshallToLibraryMessageSend())
	if err != nil {
		log.Errorf("Failed to publish message: %v", err)
		return "", err
	}

	return message.ID, nil
}

/**
 * Updates a message on discord.
 */
func (d *DiscordPublisher) UpdateMessage(version *db.DiscordMessageVersion, discordId string) error {
	_, err := d.discord.ChannelMessageEditComplex(version.MarshallToLibraryMessageEdit(d.channelId, discordId))
	if err != nil {
		log.Errorf("Failed to update message: %v", err)
		return err
	}

	return nil
}
