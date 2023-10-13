package main

import (
	db "ecfmp/discord/internal/db"
	discord "ecfmp/discord/internal/discord"
	grpc "ecfmp/discord/internal/grpc"
	"net"
	"os"

	logConfig "ecfmp/discord/internal/log"

	dotenv "github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Set the logging level
	log.SetLevel(logConfig.EnvToLogLevel(os.Getenv("LOG_LEVEL")))

	// If there's an env file in the environment variables, load it
	envFile := os.Getenv("ENV_FILE")
	if envFile != "" {
		log.Infof("Loading environment variables from %v", envFile)
		envLoadErr := dotenv.Load(envFile)
		if envLoadErr != nil {
			log.Fatalf("Failed to load environment variables from %v: %v", envFile, envLoadErr)
		}

		log.Infof("Successfully loaded environment variables from %v", envFile)
	}

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

	// Get the public key from the environment
	publicKeyFile := os.Getenv("AUTH_JWT_PUBLIC_KEY_FILE")
	if publicKeyFile == "" {
		log.Fatalf("AUTH_JWT_PUBLIC_KEY_FILE is not set")
	}

	// Get the public key by reading the file, throw error if file doesnt exist
	publicKey, err := os.ReadFile(publicKeyFile)
	if err != nil {
		log.Fatalf("failed to read public key file: %v", err)
	}
	interceptor := grpc.NewJwtAuthInterceptor(publicKey, os.Getenv("AUTH_JWT_AUDIENCE"))

	grpcServer := grpc.NewServer(mongo, scheduler, interceptor)
	log.Info("Discord server starting...")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
