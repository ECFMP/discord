package discord

import (
	discordgo "github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

type Discord interface {
	PublishMessage(content string) (string, error)
	UpdateMessage(content string, discordId string) error
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
		discord: discord,
		channelId: channelId,
	}
}

/**
  * Publishes a message to discord.
  */
func (d *DiscordPublisher) PublishMessage(content string) (string, error) {
	log.Infof("Publisher %v", d.discord)
	message, err := d.discord.ChannelMessageSend(d.channelId, content)
	if err != nil {
		log.Errorf("Failed to publish message: %v", err)
		return "", err
	}

	return message.ID, nil
}

/**
  * Updates a message on discord.
  */
func (d *DiscordPublisher) UpdateMessage(content string, discordId string) error {
	_, err := d.discord.ChannelMessageEdit(d.channelId, discordId, content)
	if err != nil {
		log.Errorf("Failed to update message: %v", err)
		return err
	}

	return nil
}
