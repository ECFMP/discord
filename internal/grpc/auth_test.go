//go:build testing
package grpc_test

import (
	"context"
	"ecfmp/discord/internal/grpc"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	metadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func SetUpTest() {
	// Set log level to fatal only
	log.SetLevel(log.FatalLevel)
}

func Test_ItPassesAuthenticationForSignedString(t *testing.T) {
	SetUpTest()

	signedJwt, err := grpc.SignJwt("test-aud", "ecfmp-auth")
	if err != nil {
		t.Fatalf("failed to sign jwt: %v", err)
	}

	authenticator, err := grpc.GetAuthenticatorWithCorrectKey("test-aud")
	if err != nil {
		t.Fatalf("failed to get authenticator: %v", err)
	}

	// Set our signed jwt in the context
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", signedJwt))

	// Call the auth interceptor and with our signed jwt and verify it passes=
	nextCalled := false
	_, err = authenticator.AuthInterceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
		nextCalled = true
		return nil, nil
	})

	if err != nil {
		t.Fatalf("failed to call auth interceptor: %v", err)
	}

	if !nextCalled {
		t.Fatalf("failed to call next handler")
	}
}

func Test_ItDoesntPassAuthenticationWrongAudience(t *testing.T) {
	SetUpTest()

	signedJwt, err := grpc.SignJwt("test-aud-2", "ecfmp-auth")
	if err != nil {
		t.Fatalf("failed to sign jwt: %v", err)
	}

	authenticator, err := grpc.GetAuthenticatorWithCorrectKey("test-aud")
	if err != nil {
		t.Fatalf("failed to get authenticator: %v", err)
	}

	// Set our signed jwt in the context
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", signedJwt))

	// Call the auth interceptor and with our signed jwt and verify it passes=
	nextCalled := false
	_, err = authenticator.AuthInterceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
		nextCalled = true
		return nil, nil
	})

	assert.Equal(t, err, status.Errorf(codes.Unauthenticated, codes.Unauthenticated.String()), nil)
	assert.False(t, nextCalled)
}

func Test_ItDoesntPassAuthenticationWrongIssuer(t *testing.T) {
	SetUpTest()

	signedJwt, err := grpc.SignJwt("test-aud", "ecfmp-auth-2")
	if err != nil {
		t.Fatalf("failed to sign jwt: %v", err)
	}

	authenticator, err := grpc.GetAuthenticatorWithCorrectKey("test-aud")
	if err != nil {
		t.Fatalf("failed to get authenticator: %v", err)
	}

	// Set our signed jwt in the context
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", signedJwt))

	// Call the auth interceptor and with our signed jwt and verify it passes=
	nextCalled := false
	_, err = authenticator.AuthInterceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
		nextCalled = true
		return nil, nil
	})

	assert.Equal(t, err, status.Errorf(codes.Unauthenticated, codes.Unauthenticated.String()), nil)
	assert.False(t, nextCalled)
}

func Test_ItDoesntPassAuthenticationEmptyAuthorization(t *testing.T) {
	SetUpTest()

	authenticator, err := grpc.GetAuthenticatorWithCorrectKey("test-aud")
	if err != nil {
		t.Fatalf("failed to get authenticator: %v", err)
	}

	// Set our signed jwt in the context
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", ""))

	// Call the auth interceptor and with our signed jwt and verify it passes=
	nextCalled := false
	_, err = authenticator.AuthInterceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
		nextCalled = true
		return nil, nil
	})

	assert.Equal(t, err, status.Errorf(codes.Unauthenticated, codes.Unauthenticated.String()), nil)
	assert.False(t, nextCalled)
}

func Test_ItDoesntPassAuthenticationNoAuthorization(t *testing.T) {
	SetUpTest()

	authenticator, err := grpc.GetAuthenticatorWithCorrectKey("test-aud")
	if err != nil {
		t.Fatalf("failed to get authenticator: %v", err)
	}

	// Call the auth interceptor and with our signed jwt and verify it passes=
	nextCalled := false
	_, err = authenticator.AuthInterceptor(context.Background(), nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
		nextCalled = true
		return nil, nil
	})

	assert.Equal(t, err, status.Errorf(codes.Unauthenticated, codes.Unauthenticated.String()), nil)
	assert.False(t, nextCalled)
}

func Test_ItDoesntPassAuthenticationSignedBySomeoneElse(t *testing.T) {
	SetUpTest()

	authenticator, err := grpc.GetAuthenticatorWithCorrectKey("test-aud")
	if err != nil {
		t.Fatalf("failed to get authenticator: %v", err)
	}

	signedJwt, err := grpc.SignJwtWithFile("test-aud", "ecfmp-auth", "../../docker/dev_private_key_2.pem")
	if err != nil {
		t.Fatalf("failed to sign jwt: %v", err)
	}

	authenticator, err := grpc.GetAuthenticatorWithCorrectKey("test-aud")
	if err != nil {
		t.Fatalf("failed to get authenticator: %v", err)
	}

	// Set our signed jwt in the context
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", signedJwt))

	// Call the auth interceptor and with our signed jwt and verify it doesnt pass
	nextCalled := false
	_, err = authenticator.AuthInterceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
		nextCalled = true
		return nil, nil
	})

	assert.Equal(t, err, status.Errorf(codes.Unauthenticated, codes.Unauthenticated.String()), nil)
	assert.False(t, nextCalled)
}
