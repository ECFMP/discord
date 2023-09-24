package grpc_test

import (
	"context"
	db "ecfmp/discord/internal/db"
	ecfmp_grpc "ecfmp/discord/internal/grpc"
	pb "ecfmp/discord/proto"
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

func SetupTest(t *testing.T) TestMongo {
	// Turn off logging except for fatals
	log.SetLevel(log.FatalLevel)

	// Mongo setup
	mongo, err := db.NewMongo()
	if err != nil {
		t.Errorf("Failed to connect to mongo: %v", err)
	}

	mongo.Client.Database("ecfmp").Collection("discord_messages").Drop(context.Background())

	// gRPC setup
	lis = bufconn.Listen(bufSize)
	s := ecfmp_grpc.NewServer(mongo)
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
	}
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
	mongo := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb.NewDiscordClient(grpcClient.conn)

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	resp, err := client.Create(ctx, &pb.DiscordMessage{Content: "Hello, world!"})

	assert.Nil(t, err)
	responseId := resp.GetId()
	mongoMessage, err := mongo.client.GetDiscordMessageById(responseId)

	assert.Nil(t, err)
	assert.Equal(t, "Hello, world!", mongoMessage.Content)
	assert.Equal(t, responseId, mongoMessage.Id)
	assert.Equal(t, "my-client-request-id", mongoMessage.ClientRequestId)
}

func Test_ItReturnsPrexistingIdIfRequestAlreadyExists(t *testing.T) {
	mongo := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb.NewDiscordClient(grpcClient.conn)

	mongoId, _ := mongo.client.WriteDiscordMessage("my-client-request-id", &pb.DiscordMessage{Content: "Hello, world!"})

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	resp, err := client.Create(ctx, &pb.DiscordMessage{Content: "Hello, world!"})

	assert.Nil(t, err)
	responseId := resp.GetId()
	assert.Equal(t, mongoId, responseId)
}

func Test_ItRejectsRequestsThatDontHaveAClientRequestId(t *testing.T) {
	mongo := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb.NewDiscordClient(grpcClient.conn)

	resp, err := client.Create(context.Background(), &pb.DiscordMessage{Content: "Hello, world!"})
	assert.Equal(t, err, status.Error(codes.InvalidArgument, "x-client-request-id metadata is required"))
	assert.Nil(t, resp)
}

func Test_ItRejectsRequestsThatDontHaveContent(t *testing.T) {
	mongo := SetupTest(t)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb.NewDiscordClient(grpcClient.conn)

	resp, err := client.Create(context.Background(), &pb.DiscordMessage{})
	assert.Equal(t, err, status.Error(codes.InvalidArgument, "Content is required"))
	assert.Nil(t, resp)
}
