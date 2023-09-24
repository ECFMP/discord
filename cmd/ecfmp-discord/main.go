package main

import (
	db "ecfmp/discord/internal/db"
	grpc "ecfmp/discord/internal/grpc"
	"fmt"
	"net"

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

	grpcServer := grpc.NewServer(mongo)
	fmt.Println("Discord server started!")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
