package grpc

import (
	"context"
	db "ecfmp/discord/internal/db"
	"ecfmp/discord/internal/discord"
	pb_discord "ecfmp/discord/proto/discord/gen/pb-go/ecfmp.vatsim.net/grpc/discord"
	pb_health "ecfmp/discord/proto/health/gen/pb-go/ecfmp.vatsim.net/grpc/health"
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	metadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// server is used to implement helloworld.GreeterServer.
type server struct {
	pb_health.UnimplementedHealthServer
	pb_discord.UnimplementedDiscordServer
	server    *grpc.Server
	mongo     *db.Mongo
	scheduler discord.Scheduler
}

/**
 * Serve the gRPC server
 */
func (server *server) Serve(listener net.Listener) error {
	return server.server.Serve(listener)
}

func getClientRequestId(ctx context.Context) (string, error) {
	metadata, ok := metadata.FromIncomingContext(ctx)

	if !ok {
		log.Error("Failed to get metadata")
		return "", fmt.Errorf("failed to get metadata from context")
	}

	if len(metadata.Get("x-client-request-id")) != 1 {
		log.Warning("Invalid request: x-client-request-id is required")
		return "", fmt.Errorf("x-client-request-id metadata is required")
	}

	if metadata.Get("x-client-request-id")[0] == "" {
		log.Warning("Invalid request: x-client-request-id is empty")
		return "", fmt.Errorf("x-client-request-id metadata is required")
	}

	return metadata.Get("x-client-request-id")[0], nil
}

func validateEmbedFields(embeds []*pb_discord.DiscordEmbeds) error {
	for _, embed := range embeds {
		for _, field := range embed.Fields {
			// Must have name, value and inline
			if field.Name == "" {
				log.Warning("Invalid request: embed name is required")
				return fmt.Errorf("embed name is required")
			}

			if field.Value == "" {
				log.Warning("Invalid request: embed value is required")
				return fmt.Errorf("embed value is required")
			}
		}
	}

	return nil
}

/**
 * Implements the Create method of the DiscordServer interface
 */
func (server *server) Create(ctx context.Context, in *pb_discord.CreateRequest) (*pb_discord.CreateResponse, error) {
	log.Debug("Create request received")

	// Check if the message has already been written, and return the existing id if so
	clientRequestId, requestIdErr := getClientRequestId(ctx)
	if requestIdErr != nil {
		return nil, status.Error(codes.InvalidArgument, requestIdErr.Error())
	}

	log.Debugf("Client request id: %v", clientRequestId)

	existingId, err := server.mongo.GetDiscordMessageByClientRequestId(clientRequestId)
	if err != nil {
		log.Errorf("Failed to get discord message by client request id: %v", err)
		return nil, status.Error(codes.Internal, "Failed to get discord message")
	}

	if existingId != nil {
		log.Infof("Discord message already exists: %v", clientRequestId)
		return &pb_discord.CreateResponse{Id: existingId.Id}, nil
	}

	// Validate that the channel is set
	if in.GetChannel() == "" {
		log.Warning("Invalid request: channel is required")
		return nil, status.Error(codes.InvalidArgument, "Channel is required")
	}

	// Validate the embed fields
	embedFieldsErr := validateEmbedFields(in.Embeds)
	if embedFieldsErr != nil {
		return nil, status.Error(codes.InvalidArgument, embedFieldsErr.Error())
	}

	// Write the message to the database
	mongoId, err := server.mongo.WriteDiscordMessage(clientRequestId, in)
	if err != nil {
		log.Errorf("Failed to write discord message: %v", err)
		return nil, status.Error(codes.Internal, "Failed to create discord message")
	}

	// Schedule the message to be published
	server.scheduler.ScheduleMessage(mongoId)

	log.Infof("Written discord message %v", mongoId)
	return &pb_discord.CreateResponse{Id: mongoId}, nil
}

/**
 * Implements the UpdateMessage of the DiscordServer proto
 */
func (server *server) Update(ctx context.Context, in *pb_discord.UpdateRequest) (*pb_discord.UpdateResponse, error) {
	if in.GetId() == "" {
		log.Warning("Invalid update request: Id is required")
		return nil, status.Error(codes.InvalidArgument, "Id is required")
	}

	// Validate the embed fields
	embedFieldsErr := validateEmbedFields(in.Embeds)
	if embedFieldsErr != nil {
		return nil, status.Error(codes.InvalidArgument, embedFieldsErr.Error())
	}

	// Check if the message has already been written, and return the existing id if so
	clientRequestId, requestIdErr := getClientRequestId(ctx)
	if requestIdErr != nil {
		return nil, status.Error(codes.InvalidArgument, requestIdErr.Error())
	}

	mongoErr := server.mongo.PublishMessageVersion(clientRequestId, in)
	if mongoErr != nil && mongoErr.Error() == "message not found" {
		log.Warning("Invalid update request: message not found")
		return nil, status.Error(codes.NotFound, codes.NotFound.String())
	}

	if mongoErr != nil {
		log.Errorf("Failed to update message: %v", mongoErr)
		return nil, status.Error(codes.Internal, "Failed to update message")
	}

	// Schedule the message update to be published
	server.scheduler.ScheduleMessage(in.Id)

	return &pb_discord.UpdateResponse{}, nil
}

/**
 * Implements the Check method of the HealthServer interface
 */
func (server *server) Check(ctx context.Context, in *pb_health.HealthCheckRequest) (*pb_health.HealthCheckResponse, error) {
	if !server.scheduler.Ready() {
		return &pb_health.HealthCheckResponse{Status: pb_health.HealthCheckResponse_NOT_SERVING}, nil
	}

	return &pb_health.HealthCheckResponse{Status: pb_health.HealthCheckResponse_SERVING}, nil
}

/**
 * Start the gRPC server
 */
func NewServer(mongo *db.Mongo, scheduler discord.Scheduler, interceptor AuthInterceptor) *server {
	s := grpc.NewServer(grpc.UnaryInterceptor(interceptor.AuthInterceptor))
	server := &server{mongo: mongo, server: s, scheduler: scheduler}
	pb_discord.RegisterDiscordServer(s, server)
	pb_health.RegisterHealthServer(s, server)

	return server
}
