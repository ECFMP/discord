package main

import (
	db "ecfmp/discord/internal/db"
	discord "ecfmp/discord/internal/discord"
	grpc "ecfmp/discord/internal/grpc"
	"fmt"
	"net"
	"os"

	log "github.com/sirupsen/logrus"
)

func main() {
	listener, err := net.Listen("tcp", ":80")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
		panic(err)
	}

	mongo, err := db.NewMongo()
	if err != nil {
		log.Fatalf("failed to connect to mongo: %v", err)
		panic(err)
	}

	// Defer closing the mongo connection
	defer func() {
		if err := mongo.Disconnect(); err != nil {
			log.Errorf("failed to disconnect from mongo: %v", err)
		}
	}()

	// Create the discord publisher
	publisher := discord.NewDiscordPublisher(os.Getenv("DISCORD_BOT_TOKEN"), os.Getenv("DISCORD_CHANNEL_ID"))

	// Create the discord scheduler
	scheduler := discord.NewDiscordScheduler(mongo, publisher)

	grpcServer := grpc.NewServer(mongo, scheduler)
	fmt.Println("Discord server started!")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
