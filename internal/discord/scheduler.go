package discord

import (
	db "ecfmp/discord/internal/db"
	log "github.com/sirupsen/logrus"
	"sync"
)

type Scheduler interface {
	ScheduleMessage(id string)
}

type DiscordScheduler struct {
	channel chan string
	mongo   *db.Mongo
	discord Discord

	GoRoutineWaitGroup *sync.WaitGroup
}

/**
 * Creates a new discord scheduler.
 */
func NewDiscordScheduler(mongo *db.Mongo, discordInterface Discord) *DiscordScheduler {
	scheduler := &DiscordScheduler{
		mongo:              mongo,
		discord:            discordInterface,
		channel:            make(chan string, 50),
		GoRoutineWaitGroup: &sync.WaitGroup{},
	}

	go func(schedulerToProcess *DiscordScheduler) {
		schedulerToProcess.processChannel()
	}(scheduler)

	return scheduler
}

/**
 * Schedules a message to be published to discord.
 */
func (d *DiscordScheduler) ScheduleMessage(id string) {
	log.Infof("Scheduler: Scheduling message with id %v", id)
	d.GoRoutineWaitGroup.Add(1)
	d.channel <- id
}

/**
 * Called by the scheduler's goroutine to process messages from the channel asynchonously to the
 *	request that scheduled them.
 */
func (d *DiscordScheduler) processChannel() {
	log.Infof("Started discord scheduler routine")
	for msg := range d.channel {
		log.Infof("Scheduler: Processing message %v", msg)

		mongoMessage, mongoErr := d.mongo.GetDiscordMessageById(msg)
		if mongoErr != nil {
			log.Errorf("Scheduler: Failed to get message from mongo to publish: %v", mongoErr)
			continue
		}

		if mongoMessage == nil {
			log.Errorf("Scheduler: Message not found in mongo for publishing: %v", msg)
			continue
		}

		// If the message has no discord id, publish it as a new message. Otherwise, update the existing message.
		if mongoMessage.DiscordId == "" {
			publishNewMessage(d, mongoMessage)
		} else {
			publishMessageUpdate(d, mongoMessage)
		}

		d.GoRoutineWaitGroup.Done()
	}
}

/**
 * Publishes a new message to discord and updates the message in mongo to have the discord id.
 */
func publishNewMessage(d *DiscordScheduler, mongoMessage *db.DiscordMessage) {
	versionToPublish := &mongoMessage.Versions[len(mongoMessage.Versions)-1]
	discordId, publishErr := d.discord.PublishMessage(versionToPublish)

	if publishErr != nil {
		log.Errorf("Scheduler: Failed to publish message to discord: %v", publishErr)
		return
	}

	mongoMessage.DiscordId = discordId
	mongoMessage.LastClientRequestPublished = versionToPublish.ClientRequestId
	mongoErr := d.mongo.UpdateMessageWithDiscordIdAndLastPublishRequest(mongoMessage.Id, discordId, versionToPublish.ClientRequestId)
	if mongoErr != nil {
		log.Errorf("Scheduler: Failed to update message in mongo for publish: %v", mongoErr)
		return
	}

	log.Infof("Published new message with client request id %v as %v", versionToPublish.ClientRequestId, discordId)
}

/**
 * Updates an existing message in discord and mongo.
 */
func publishMessageUpdate(d *DiscordScheduler, mongoMessage *db.DiscordMessage) {
	versionToPublish := &mongoMessage.Versions[len(mongoMessage.Versions)-1]
	updateErr := d.discord.UpdateMessage(versionToPublish, mongoMessage.DiscordId)
	if updateErr != nil {
		log.Errorf("Scheduler: Failed to update message: %v", updateErr)
		return
	}

	mongoMessage.LastClientRequestPublished = versionToPublish.ClientRequestId
	mongoErr := d.mongo.UpdateMessageWithLastPublishRequest(mongoMessage.Id, versionToPublish.ClientRequestId)
	if mongoErr != nil {
		log.Errorf("Scheduler: Failed to update message in mongo: %v", mongoErr)
		return
	}

	log.Infof("Scheduler: Updated message %v with client request id %v", mongoMessage.DiscordId, versionToPublish.ClientRequestId)
}
