package grpc_test

import (
	"context"
	db "ecfmp/discord/internal/db"
	ecfmp_grpc "ecfmp/discord/internal/grpc"
	pb_discord "ecfmp/discord/proto/discord"
	pb_health "ecfmp/discord/proto/health"
	"net"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// Buffer size for the gRPC server
const bufSize = 1024 * 1024

var lis *bufconn.Listener

type TestMongo struct {
	client   *db.Mongo
	tearDown func()
}

type MockScheduler struct {
	callCount int
	callId string
}

func (scheduler *MockScheduler) ScheduleMessage(id string) {
	scheduler.callCount++
	scheduler.callId = id
}

func SetupTest(t *testing.T) (TestMongo, *MockScheduler) {
	// Turn off logging except for fatals
	log.SetLevel(log.FatalLevel)

	// Mongo setup
	mongo, err := db.NewMongo()
	if err != nil {
		t.Errorf("Failed to connect to mongo: %v", err)
	}

	mongo.Client.Database("ecfmp_test").Collection("discord_messages").Drop(context.Background())

	// Mock scheduler
	scheduler := &MockScheduler{}

	// gRPC setup
	lis = bufconn.Listen(bufSize)
	s := ecfmp_grpc.NewServer(mongo, scheduler)
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()

	return TestMongo{
		client: mongo,
		tearDown: func() {
			mongo.Client.Disconnect(context.Background())
		},
	}, scheduler
}

func dialBuffer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

type TestClient struct {
	conn  *grpc.ClientConn
	close func()
}

func setupGrpcClient() TestClient {
	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithContextDialer(dialBuffer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		log.Fatalf("Failed to dial bufnet: %v", err)
	}

	return TestClient{
		conn: conn,
		close: func() {
			conn.Close()
		},
	}
}

func Test_ItCreatesADiscordMessage(t *testing.T) {
	mongo, scheduler := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	resp, err := client.Create(ctx, &pb_discord.CreateRequest{Content: "Hello, world!"})

	assert.Nil(t, err)
	responseId := resp.GetId()
	mongoMessage, err := mongo.client.GetDiscordMessageById(responseId)

	assert.Nil(t, err)
	assert.Equal(t, responseId, mongoMessage.Id)
	assert.Equal(t, 1, len(mongoMessage.Versions))
	assert.Equal(t, "my-client-request-id", mongoMessage.Versions[0].ClientRequestId)
	assert.Equal(t, "Hello, world!", mongoMessage.Versions[0].Content)

	assert.Equal(t, 1, scheduler.callCount)
	assert.Equal(t, responseId, scheduler.callId)
}

func Test_ItReturnsPrexistingIdIfRequestAlreadyExists(t *testing.T) {
	mongo, scheduler := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	mongoId, _ := mongo.client.WriteDiscordMessage("my-client-request-id", &pb_discord.CreateRequest{Content: "Hello, world!"})

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	resp, err := client.Create(ctx, &pb_discord.CreateRequest{Content: "Hello, world!"})

	assert.Nil(t, err)
	responseId := resp.GetId()
	assert.Equal(t, mongoId, responseId)

	assert.Equal(t, 0, scheduler.callCount)
}

func Test_ItRejectsRequestsThatDontHaveAClientRequestId(t *testing.T) {
	mongo, scheduler := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	resp, err := client.Create(context.Background(), &pb_discord.CreateRequest{Content: "Hello, world!"})
	assert.Equal(t, err, status.Error(codes.InvalidArgument, "x-client-request-id metadata is required"))
	assert.Nil(t, resp)

	assert.Equal(t, 0, scheduler.callCount)
}

func Test_ItRejectsRequestsThatHaveAnEmptyClientRequestId(t *testing.T) {
	mongo, scheduler := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	grpcMetadata := metadata.Pairs("x-client-request-id", "")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	resp, err := client.Create(ctx, &pb_discord.CreateRequest{Content: "Hello, world!"})
	assert.Equal(t, err, status.Error(codes.InvalidArgument, "x-client-request-id metadata is required"))
	assert.Nil(t, resp)

	assert.Equal(t, 0, scheduler.callCount)
}

func Test_ItRejectsRequestsThatDontHaveContent(t *testing.T) {
	mongo, scheduler := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	resp, err := client.Create(context.Background(), &pb_discord.CreateRequest{})
	assert.Equal(t, err, status.Error(codes.InvalidArgument, "Content is required"))
	assert.Nil(t, resp)

	assert.Equal(t, 0, scheduler.callCount)
}

func Test_ItDoesAHealthCheck(t *testing.T) {
	mongo, _ := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_health.NewHealthClient(grpcClient.conn)

	resp, err := client.Check(context.Background(), &pb_health.HealthCheckRequest{})
	assert.Nil(t, err)
	assert.Equal(t, resp.Status, pb_health.HealthCheckResponse_SERVING)
}

func Test_ItUpdatesAMessage(t *testing.T) {
	mongo, scheduler := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	mongoId, _ := mongo.client.WriteDiscordMessage("my-client-request-id", &pb_discord.CreateRequest{Content: "Hello, world!"})

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id-2")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	_, err := client.Update(ctx, &pb_discord.UpdateRequest{Id: mongoId, Content: "Hello, world, again!"})

	assert.Nil(t, err)

	// Then
	mongoMessage, err := mongo.client.GetDiscordMessageById(mongoId)
	assert.Nil(t, err)
	assert.Equal(t, mongoId, mongoMessage.Id)
	assert.Equal(t, 2, len(mongoMessage.Versions))
	assert.Equal(t, "my-client-request-id", mongoMessage.Versions[0].ClientRequestId)
	assert.Equal(t, "Hello, world!", mongoMessage.Versions[0].Content)
	assert.Equal(t, "my-client-request-id-2", mongoMessage.Versions[1].ClientRequestId)
	assert.Equal(t, "Hello, world, again!", mongoMessage.Versions[1].Content)

	assert.Equal(t, 1, scheduler.callCount)
	assert.Equal(t, mongoId, scheduler.callId)
}

func Test_ItDoesntUpdateAMessageNotFound(t *testing.T) {
	mongo, scheduler := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	mongo.client.WriteDiscordMessage("my-client-request-id", &pb_discord.CreateRequest{Content: "Hello, world!"})

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id-2")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	_, err := client.Update(ctx, &pb_discord.UpdateRequest{Id: "65106dab41199f298668474f", Content: "Hello, world, again!"})

	assert.NotNil(t, err)
	assert.Equal(t, status.Error(codes.NotFound, codes.NotFound.String()), err)

	assert.Equal(t, 0, scheduler.callCount)
}

func Test_ItDoesntUpdateAMessageClientRequestIdEmpty(t *testing.T) {
	mongo, scheduler := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	mongo.client.WriteDiscordMessage("my-client-request-id", &pb_discord.CreateRequest{Content: "Hello, world!"})

	grpcMetadata := metadata.Pairs("x-client-request-id", "")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	_, err := client.Update(ctx, &pb_discord.UpdateRequest{Id: "65106dab41199f298668474f", Content: "Hello, world, again!"})

	assert.NotNil(t, err)
	assert.Equal(t, status.Error(codes.InvalidArgument, "x-client-request-id metadata is required"), err)

	assert.Equal(t, 0, scheduler.callCount)
}

func Test_ItDoesntUpdateAMessageClientRequestIdMissing(t *testing.T) {
	mongo, scheduler := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	mongo.client.WriteDiscordMessage("my-client-request-id", &pb_discord.CreateRequest{Content: "Hello, world!"})

	_, err := client.Update(context.Background(), &pb_discord.UpdateRequest{Id: "65106dab41199f298668474f", Content: "Hello, world, again!"})

	assert.NotNil(t, err)
	assert.Equal(t, status.Error(codes.InvalidArgument, "x-client-request-id metadata is required"), err)

	assert.Equal(t, 0, scheduler.callCount)
}

func Test_ItCanCreateThenUpdateAMessage(t *testing.T) {
	mongo, scheduler := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	resp, err := client.Create(ctx, &pb_discord.CreateRequest{Content: "Hello, world!"})

	assert.Nil(t, err)
	responseId := resp.GetId()
	mongoMessage, err := mongo.client.GetDiscordMessageById(responseId)

	assert.Nil(t, err)
	assert.Equal(t, responseId, mongoMessage.Id)
	assert.Equal(t, 1, len(mongoMessage.Versions))
	assert.Equal(t, "my-client-request-id", mongoMessage.Versions[0].ClientRequestId)
	assert.Equal(t, "Hello, world!", mongoMessage.Versions[0].Content)

	assert.Equal(t, 1, scheduler.callCount)
	assert.Equal(t, responseId, scheduler.callId)

	grpcMetadata = metadata.Pairs("x-client-request-id", "my-client-request-id-2")
	ctx = metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	_, err = client.Update(ctx, &pb_discord.UpdateRequest{Id: responseId, Content: "Hello, world, again!"})

	assert.Nil(t, err)

	mongoMessage, err = mongo.client.GetDiscordMessageById(responseId)
	assert.Nil(t, err)
	assert.Equal(t, responseId, mongoMessage.Id)
	assert.Equal(t, 2, len(mongoMessage.Versions))
	assert.Equal(t, "my-client-request-id", mongoMessage.Versions[0].ClientRequestId)
	assert.Equal(t, "Hello, world!", mongoMessage.Versions[0].Content)
	assert.Equal(t, "my-client-request-id-2", mongoMessage.Versions[1].ClientRequestId)
	assert.Equal(t, "Hello, world, again!", mongoMessage.Versions[1].Content)

	assert.Equal(t, 2, scheduler.callCount)
	assert.Equal(t, responseId, scheduler.callId)
}
