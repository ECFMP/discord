package grpc

import (
	"context"
	"crypto/rsa"
	grpc_health "ecfmp/discord/proto/health"

	log "github.com/sirupsen/logrus"

	jwt "github.com/golang-jwt/jwt/v5"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	metadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type AuthInterceptor interface {
	AuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error)
}

type JwtAuthInterceptor struct {
	publicKey   *rsa.PublicKey
	keyAudience string
}

type NullInterceptor struct{}

func NewJwtAuthInterceptor(publicKey []byte, keyAudience string) *JwtAuthInterceptor {
	publicKeyFromPem, err := jwt.ParseRSAPublicKeyFromPEM(publicKey)
	if err != nil {
		panic(err)
	}

	return &JwtAuthInterceptor{
		publicKey:   publicKeyFromPem,
		keyAudience: keyAudience,
	}
}

func NewNullInterceptor() *NullInterceptor {
	return &NullInterceptor{}
}

func (interceptor *NullInterceptor) AuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return handler(ctx, req)
}

func (interceptor *JwtAuthInterceptor) validateJwt(passedJwt string) (bool, error) {
	token, err := jwt.Parse(passedJwt, func(token *jwt.Token) (interface{}, error) {
		return interceptor.publicKey, nil
	}, jwt.WithAudience(interceptor.keyAudience), jwt.WithIssuer("ecfmp-auth"))

	if err != nil {
		return false, err
	}

	if !token.Valid {
		return false, status.Errorf(codes.Unauthenticated, "invalid token")
	}

	return true, nil
}

func (interceptor *JwtAuthInterceptor) AuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Check the request type, if its healthcheck, no auth required
	switch req.(type) {
	case *grpc_health.HealthCheckRequest:
		return handler(ctx, req)
	}

	// Get the JWT from the request metadata
	metadata, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, codes.Unauthenticated.String())
	}

	if len(metadata.Get("authorization")) != 1 {
		log.Warn("authorization metadata is required")
		return nil, status.Error(codes.Unauthenticated, codes.Unauthenticated.String())
	}

	if metadata.Get("authorization")[0] == "" {
		log.Warn("authorization metadata is required")
		return nil, status.Error(codes.Unauthenticated, codes.Unauthenticated.String())
	}

	// Validate the JWT
	valid, err := interceptor.validateJwt(metadata.Get("authorization")[0])
	if err != nil {
		log.Warn("failed to validate jwt: ", err)
		return nil, status.Error(codes.Unauthenticated, codes.Unauthenticated.String())
	}

	if !valid {
		log.Warn("invalid token")
		return nil, status.Error(codes.Unauthenticated, codes.Unauthenticated.String())
	}

	// Call the handler with a new context
	return handler(ctx, req)
}
