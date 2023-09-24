package grpc

import (
	"context"
	db "ecfmp/discord/internal/db"
	pb "ecfmp/discord/proto"
	"net"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	metadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// server is used to implement helloworld.GreeterServer.
type server struct {
	pb.UnimplementedDiscordServer
	server *grpc.Server
	mongo  *db.Mongo
}

/**
 * Serve the gRPC server
 */
func (server *server) Serve(listener net.Listener) error {
	return server.server.Serve(listener)
}

/**
 * Implements the Create method of the DiscordServer interface
 */
func (server *server) Create(ctx context.Context, in *pb.DiscordMessage) (*pb.CreateResponse, error) {
	if in.GetContent() == "" {
		log.Warning("Invalid request: Content is required")
		return nil, status.Error(codes.InvalidArgument, "Content is required")
	}

	metadata, ok := metadata.FromIncomingContext(ctx)

	if !ok {
		log.Error("Failed to get metadata")
		return nil, status.Error(codes.Internal, "Failed to get metadata from context")
	}

	if len(metadata.Get("x-client-request-id")) != 1 {
		log.Warning("Invalid request: x-client-request-id is required")
		return nil, status.Error(codes.InvalidArgument, "x-client-request-id metadata is required")
	}

	// Check if the message has already been written, and return the existing id if so
	existingId, err := server.mongo.GetDiscordMessageByClientRequestId(metadata.Get("x-client-request-id")[0])
	if err != nil {
		log.Errorf("Failed to get discord message by client request id: %v", err)
		return nil, status.Error(codes.Internal, "Failed to get discord message")
	}

	if existingId != nil && existingId.ClientRequestId != "" {
		log.Infof("Discord message already exists: %v", existingId.ClientRequestId)
		return &pb.CreateResponse{Id: existingId.Id}, nil
	}

	// Write the message to the database
	mongoId, err := server.mongo.WriteDiscordMessage(metadata.Get("x-client-request-id")[0], in)
	if err != nil {
		log.Errorf("Failed to write discord message: %v", err)
		return nil, status.Error(codes.Internal, "Failed to create discord message")
	}

	log.Infof("Written discord message %v", mongoId)

	return &pb.CreateResponse{Id: mongoId}, nil
}

/**
 * Start the gRPC server
 */
func NewServer(mongo *db.Mongo) *server {
	s := grpc.NewServer()
	server := &server{mongo: mongo, server: s}
	pb.RegisterDiscordServer(s, server)

	return server
}
