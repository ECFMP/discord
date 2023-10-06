package grpc_test

import (
	"context"
	db "ecfmp/discord/internal/db"
	ecfmp_grpc "ecfmp/discord/internal/grpc"
	pb_discord "ecfmp/discord/proto/discord/gen/pb-go/ecfmp.vatsim.net/grpc/discord"
	pb_health "ecfmp/discord/proto/health/gen/pb-go/ecfmp.vatsim.net/grpc/health"
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
	callId    string
}

func (scheduler *MockScheduler) ScheduleMessage(id string) {
	scheduler.callCount++
	scheduler.callId = id
}

func SetupTest(t *testing.T, realInterceptor bool) (TestMongo, *MockScheduler) {
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

	var interceptor ecfmp_grpc.AuthInterceptor
	if realInterceptor {
		publicKeyBytes, err := GetPublicKeyBytes()
		if err != nil {
			t.Errorf("Failed to get public key bytes: %v", err)
		}

		interceptor = ecfmp_grpc.NewJwtAuthInterceptor(publicKeyBytes, "test-aud")
	} else {
		interceptor = ecfmp_grpc.NewNullInterceptor()
	}

	// gRPC setup
	lis = bufconn.Listen(bufSize)
	s := ecfmp_grpc.NewServer(mongo, scheduler, interceptor)
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
	mongo, scheduler := SetupTest(t, false)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	resp, err := client.Create(
		ctx,
		&pb_discord.CreateRequest{
			Content: "Hello World!",
			Embeds: []*pb_discord.DiscordEmbeds{
				{
					Title:       "Hello World!",
					Description: "This is a test",
					Url:         "https://example.com",
					Color:       123456,
					Fields: []*pb_discord.DiscordEmbedsFields{
						{
							Name:   "Field 1",
							Value:  "Value 1",
							Inline: true,
						},
						{
							Name:   "Field 2",
							Value:  "Value 2",
							Inline: false,
						},
					},
				},
			},
		})

	assert.Nil(t, err)
	responseId := resp.GetId()
	mongoMessage, err := mongo.client.GetDiscordMessageById(responseId)

	assert.Nil(t, err)
	assert.Equal(t, responseId, mongoMessage.Id)
	assert.Equal(t, 1, len(mongoMessage.Versions))
	assert.Equal(t, "my-client-request-id", mongoMessage.Versions[0].ClientRequestId)
	assert.Equal(t, "Hello World!", mongoMessage.Versions[0].Content)
	assert.Equal(t, "Hello World!", mongoMessage.Versions[0].Embeds[0].Title)
	assert.Equal(t, "This is a test", mongoMessage.Versions[0].Embeds[0].Description)
	assert.Equal(t, "https://example.com", mongoMessage.Versions[0].Embeds[0].Url)
	assert.Equal(t, int32(123456), mongoMessage.Versions[0].Embeds[0].Color)
	assert.Equal(t, "Field 1", mongoMessage.Versions[0].Embeds[0].Fields[0].Name)
	assert.Equal(t, "Value 1", mongoMessage.Versions[0].Embeds[0].Fields[0].Value)
	assert.Equal(t, true, mongoMessage.Versions[0].Embeds[0].Fields[0].Inline)
	assert.Equal(t, "Field 2", mongoMessage.Versions[0].Embeds[0].Fields[1].Name)
	assert.Equal(t, "Value 2", mongoMessage.Versions[0].Embeds[0].Fields[1].Value)
	assert.Equal(t, false, mongoMessage.Versions[0].Embeds[0].Fields[1].Inline)

	assert.Equal(t, 1, scheduler.callCount)
	assert.Equal(t, responseId, scheduler.callId)
}

func Test_ItAllowsCreateIfAuthenticated(t *testing.T) {
	mongo, _ := SetupTest(t, true)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	token, err := SignJwt("test-aud", "ecfmp-auth")
	assert.Nil(t, err)

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id", "authorization", token)
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	resp, err := client.Create(
		ctx,
		&pb_discord.CreateRequest{
			Content: "Hello World!",
			Embeds: []*pb_discord.DiscordEmbeds{
				{
					Title:       "Hello World!",
					Description: "This is a test",
					Url:         "https://example.com",
					Color:       123456,
					Fields: []*pb_discord.DiscordEmbedsFields{
						{
							Name:   "Field 1",
							Value:  "Value 1",
							Inline: true,
						},
						{
							Name:   "Field 2",
							Value:  "Value 2",
							Inline: false,
						},
					},
				},
			},
		})

	assert.Nil(t, err)
	responseId := resp.GetId()
	mongoMessage, err := mongo.client.GetDiscordMessageById(responseId)

	assert.Nil(t, err)
	assert.Equal(t, responseId, mongoMessage.Id)
}

func Test_ItForbidsCreateIfNotAuthenticated(t *testing.T) {
	mongo, _ := SetupTest(t, true)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	// Token is signed with a different key
	token, err := SignJwtWithFile("test-aud", "ecfmp-auth", "../../docker/dev_private_key_2.pem")
	assert.Nil(t, err)

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id", "authorization", token)
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	resp, err := client.Create(
		ctx,
		&pb_discord.CreateRequest{
			Content: "Hello World!",
			Embeds: []*pb_discord.DiscordEmbeds{
				{
					Title:       "Hello World!",
					Description: "This is a test",
					Url:         "https://example.com",
					Color:       123456,
					Fields: []*pb_discord.DiscordEmbedsFields{
						{
							Name:   "Field 1",
							Value:  "Value 1",
							Inline: true,
						},
						{
							Name:   "Field 2",
							Value:  "Value 2",
							Inline: false,
						},
					},
				},
			},
		})

	assert.NotNil(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, status.Error(codes.Unauthenticated, codes.Unauthenticated.String()), err)
}

func Test_ItReturnsPrexistingIdIfRequestAlreadyExists(t *testing.T) {
	mongo, scheduler := SetupTest(t, false)
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
	mongo, scheduler := SetupTest(t, false)
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
	mongo, scheduler := SetupTest(t, false)
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
	mongo, scheduler := SetupTest(t, false)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	resp, err := client.Create(context.Background(), &pb_discord.CreateRequest{})
	assert.Equal(t, err, status.Error(codes.InvalidArgument, "Content is required"))
	assert.Nil(t, resp)

	assert.Equal(t, 0, scheduler.callCount)
}

func Test_ItRejectsAMessageThatHasMissingFieldName(t *testing.T) {
	mongo, scheduler := SetupTest(t, false)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	resp, err := client.Create(
		ctx,
		&pb_discord.CreateRequest{
			Content: "Hello World!",
			Embeds: []*pb_discord.DiscordEmbeds{
				{
					Title:       "Hello World!",
					Description: "This is a test",
					Url:         "https://example.com",
					Color:       123456,
					Fields: []*pb_discord.DiscordEmbedsFields{
						{
							Name:   "",
							Value:  "Value 1",
							Inline: true,
						},
						{
							Name:   "Field 2",
							Value:  "Value 2",
							Inline: false,
						},
					},
				},
			},
		})

	assert.Equal(t, err, status.Error(codes.InvalidArgument, "embed name is required"))
	assert.Nil(t, resp)

	assert.Equal(t, 0, scheduler.callCount)
}

func Test_ItRejectsAMessageThatHasMissingFieldValue(t *testing.T) {
	mongo, scheduler := SetupTest(t, false)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	resp, err := client.Create(
		ctx,
		&pb_discord.CreateRequest{
			Content: "Hello World!",
			Embeds: []*pb_discord.DiscordEmbeds{
				{
					Title:       "Hello World!",
					Description: "This is a test",
					Url:         "https://example.com",
					Color:       123456,
					Fields: []*pb_discord.DiscordEmbedsFields{
						{
							Name:   "Field 1",
							Value:  "Value 1",
							Inline: true,
						},
						{
							Name:   "Field 2",
							Value:  "",
							Inline: false,
						},
					},
				},
			},
		})

	assert.Equal(t, err, status.Error(codes.InvalidArgument, "embed value is required"))
	assert.Nil(t, resp)

	assert.Equal(t, 0, scheduler.callCount)
}

func Test_ItDoesAHealthCheck(t *testing.T) {
	mongo, _ := SetupTest(t, false)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_health.NewHealthClient(grpcClient.conn)

	resp, err := client.Check(context.Background(), &pb_health.HealthCheckRequest{})
	assert.Nil(t, err)
	assert.Equal(t, resp.Status, pb_health.HealthCheckResponse_SERVING)
}

func Test_ItDoesAHealthCheckIfUnauthenticated(t *testing.T) {
	mongo, _ := SetupTest(t, true)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_health.NewHealthClient(grpcClient.conn)

	resp, err := client.Check(context.Background(), &pb_health.HealthCheckRequest{})
	assert.Nil(t, err)
	assert.Equal(t, resp.Status, pb_health.HealthCheckResponse_SERVING)
}

func Test_ItUpdatesAMessage(t *testing.T) {
	mongo, scheduler := SetupTest(t, false)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	mongoId, _ := mongo.client.WriteDiscordMessage("my-client-request-id", &pb_discord.CreateRequest{Content: "Hello, world!"})

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id-2")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	_, err := client.Update(
		ctx,
		&pb_discord.UpdateRequest{
			Id:      mongoId,
			Content: "Hello, world, again!",

			Embeds: []*pb_discord.DiscordEmbeds{
				{
					Title:       "Hello World!",
					Description: "This is a test",
					Url:         "https://example.com",
					Color:       123456,
					Fields: []*pb_discord.DiscordEmbedsFields{
						{
							Name:   "Field 1",
							Value:  "Value 1",
							Inline: true,
						},
						{
							Name:   "Field 2",
							Value:  "Value 2",
							Inline: false,
						},
					},
				},
			},
		},
	)

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
	assert.Equal(t, "Hello World!", mongoMessage.Versions[1].Embeds[0].Title)
	assert.Equal(t, "This is a test", mongoMessage.Versions[1].Embeds[0].Description)
	assert.Equal(t, "https://example.com", mongoMessage.Versions[1].Embeds[0].Url)
	assert.Equal(t, int32(123456), mongoMessage.Versions[1].Embeds[0].Color)
	assert.Equal(t, "Field 1", mongoMessage.Versions[1].Embeds[0].Fields[0].Name)
	assert.Equal(t, "Value 1", mongoMessage.Versions[1].Embeds[0].Fields[0].Value)
	assert.Equal(t, true, mongoMessage.Versions[1].Embeds[0].Fields[0].Inline)
	assert.Equal(t, "Field 2", mongoMessage.Versions[1].Embeds[0].Fields[1].Name)
	assert.Equal(t, "Value 2", mongoMessage.Versions[1].Embeds[0].Fields[1].Value)
	assert.Equal(t, false, mongoMessage.Versions[1].Embeds[0].Fields[1].Inline)

	assert.Equal(t, 1, scheduler.callCount)
	assert.Equal(t, mongoId, scheduler.callId)
}

func Test_ItUpdatesAMessageIfAuthenticated(t *testing.T) {
	mongo, _ := SetupTest(t, true)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	mongoId, _ := mongo.client.WriteDiscordMessage("my-client-request-id", &pb_discord.CreateRequest{Content: "Hello, world!"})

	token, tokenErr := SignJwt("test-aud", "ecfmp-auth")
	assert.Nil(t, tokenErr)

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id-2", "authorization", token)
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	_, err := client.Update(
		ctx,
		&pb_discord.UpdateRequest{
			Id:      mongoId,
			Content: "Hello, world, again!",

			Embeds: []*pb_discord.DiscordEmbeds{
				{
					Title:       "Hello World!",
					Description: "This is a test",
					Url:         "https://example.com",
					Color:       123456,
					Fields: []*pb_discord.DiscordEmbedsFields{
						{
							Name:   "Field 1",
							Value:  "Value 1",
							Inline: true,
						},
						{
							Name:   "Field 2",
							Value:  "Value 2",
							Inline: false,
						},
					},
				},
			},
		},
	)

	assert.Nil(t, err)

	// Then
	mongoMessage, err := mongo.client.GetDiscordMessageById(mongoId)
	assert.Nil(t, err)
	assert.Equal(t, mongoId, mongoMessage.Id)
	assert.Equal(t, 2, len(mongoMessage.Versions))
}

func Test_ItRejectsAMessageIfNotAuthenticated(t *testing.T) {
	mongo, _ := SetupTest(t, true)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	mongoId, _ := mongo.client.WriteDiscordMessage("my-client-request-id", &pb_discord.CreateRequest{Content: "Hello, world!"})

	// Token is signed with a different key
	token, tokenErr := SignJwtWithFile("test-aud", "ecfmp-auth", "../../docker/dev_private_key_2.pem")
	assert.Nil(t, tokenErr)

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id-2", "authorization", token)
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	_, err := client.Update(
		ctx,
		&pb_discord.UpdateRequest{
			Id:      mongoId,
			Content: "Hello, world, again!",

			Embeds: []*pb_discord.DiscordEmbeds{
				{
					Title:       "Hello World!",
					Description: "This is a test",
					Url:         "https://example.com",
					Color:       123456,
					Fields: []*pb_discord.DiscordEmbedsFields{
						{
							Name:   "Field 1",
							Value:  "Value 1",
							Inline: true,
						},
						{
							Name:   "Field 2",
							Value:  "Value 2",
							Inline: false,
						},
					},
				},
			},
		},
	)

	assert.NotNil(t, err)
	assert.Equal(t, status.Error(codes.Unauthenticated, codes.Unauthenticated.String()), err)
}

func Test_ItDoesntUpdateAMessageNotFound(t *testing.T) {
	mongo, scheduler := SetupTest(t, false)
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

func Test_ItDoesntUpdateAMessageNoContent(t *testing.T) {
	mongo, scheduler := SetupTest(t, false)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	mongoId, _ := mongo.client.WriteDiscordMessage("my-client-request-id", &pb_discord.CreateRequest{Content: "Hello, world!"})

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id-2")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	_, err := client.Update(ctx, &pb_discord.UpdateRequest{Id: mongoId, Content: ""})

	assert.NotNil(t, err)
	assert.Equal(t, status.Error(codes.InvalidArgument, "Content is required"), err)

	assert.Equal(t, 0, scheduler.callCount)
}

func Test_ItDoesntUpdateAMessageNoIdSpecified(t *testing.T) {
	mongo, scheduler := SetupTest(t, false)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	mongo.client.WriteDiscordMessage("my-client-request-id", &pb_discord.CreateRequest{Content: "Hello, world!"})

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id-2")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	_, err := client.Update(ctx, &pb_discord.UpdateRequest{Id: "", Content: "abc"})

	assert.NotNil(t, err)
	assert.Equal(t, status.Error(codes.InvalidArgument, "Id is required"), err)

	assert.Equal(t, 0, scheduler.callCount)
}

func Test_ItRejectsAnUpdateThatHasMissingFieldName(t *testing.T) {
	mongo, scheduler := SetupTest(t, false)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	mongoId, _ := mongo.client.WriteDiscordMessage("my-client-request-id", &pb_discord.CreateRequest{Content: "Hello, world!"})

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	resp, err := client.Update(
		ctx,
		&pb_discord.UpdateRequest{
			Id:      mongoId,
			Content: "Hello, world, again!",
			Embeds: []*pb_discord.DiscordEmbeds{
				{
					Title:       "Hello World!",
					Description: "This is a test",
					Url:         "https://example.com",
					Color:       123456,
					Fields: []*pb_discord.DiscordEmbedsFields{
						{
							Name:   "Field 1",
							Value:  "Value 1",
							Inline: true,
						},
						{
							Name:   "",
							Value:  "Value 2",
							Inline: false,
						},
					},
				},
			},
		},
	)

	assert.Equal(t, err, status.Error(codes.InvalidArgument, "embed name is required"))
	assert.Nil(t, resp)

	assert.Equal(t, 0, scheduler.callCount)
}

func Test_ItRejectsAnUpdateThatHasMissingFieldValue(t *testing.T) {
	mongo, scheduler := SetupTest(t, false)
	defer mongo.tearDown()

	grpcClient := setupGrpcClient()
	defer grpcClient.close()

	mongoId, _ := mongo.client.WriteDiscordMessage("my-client-request-id", &pb_discord.CreateRequest{Content: "Hello, world!"})

	client := pb_discord.NewDiscordClient(grpcClient.conn)

	grpcMetadata := metadata.Pairs("x-client-request-id", "my-client-request-id")
	ctx := metadata.NewOutgoingContext(context.Background(), grpcMetadata)
	resp, err := client.Update(
		ctx,
		&pb_discord.UpdateRequest{
			Id:      mongoId,
			Content: "Hello, world, again!",
			Embeds: []*pb_discord.DiscordEmbeds{
				{
					Title:       "Hello World!",
					Description: "This is a test",
					Url:         "https://example.com",
					Color:       123456,
					Fields: []*pb_discord.DiscordEmbedsFields{
						{
							Name:   "Field 1",
							Value:  "",
							Inline: true,
						},
						{
							Name:   "Field 2",
							Value:  "Value 2",
							Inline: false,
						},
					},
				},
			},
		},
	)

	assert.Equal(t, err, status.Error(codes.InvalidArgument, "embed value is required"))
	assert.Nil(t, resp)

	assert.Equal(t, 0, scheduler.callCount)
}

func Test_ItDoesntUpdateAMessageClientRequestIdEmpty(t *testing.T) {
	mongo, scheduler := SetupTest(t, false)
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
	mongo, scheduler := SetupTest(t, false)
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
	mongo, scheduler := SetupTest(t, false)
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
