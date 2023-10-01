package grpc_test

import (
	"log"
	"os"

	grpc "ecfmp/discord/internal/grpc"
	"github.com/golang-jwt/jwt/v5"
)

func SignJwt(audience string, issuer string) (string, error) {
	return SignJwtWithFile(audience, issuer, "../../docker/dev_private_key.pem")
}

func SignJwtWithFile(audience string, issuer string, filePath string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"aud": audience,
		"iss": issuer,
	})

	privateKey, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("failed to read private key file: %v", err)
	}

	privateKeyPem, err := jwt.ParseRSAPrivateKeyFromPEM(privateKey)
	if err != nil {
		log.Fatalf("failed to parse private key: %v", err)
	}

	return token.SignedString(privateKeyPem)
}

func GetPublicKeyBytes() ([]byte, error) {
	publicKey, err := os.ReadFile("../../docker/dev_public_key.pub")
	if err != nil {
		log.Fatalf("failed to read public key file: %v", err)
	}

	return publicKey, nil
}

func GetPublicKeyString() (string, error) {
	publicKeyBytes, _err := GetPublicKeyBytes()
	if _err != nil {
		return "", _err
	}

	return string(publicKeyBytes), nil
}

func GetAuthenticatorWithCorrectKey(audience string) (*grpc.JwtAuthInterceptor, error) {
	publicKey, err := GetPublicKeyBytes()
	if err != nil {
		log.Fatalf("failed to read public key file: %v", err)
	}

	return grpc.NewJwtAuthInterceptor(publicKey, audience), nil
}
