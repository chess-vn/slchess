package server

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/chess-vn/slchess/pkg/logging"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// Struct for Cognito's JWKS JSON response
type jwk struct {
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwks struct {
	Keys []jwk `json:"keys"`
}

func (s *server) loadCognitoPublicKeys() {
	url := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json", s.config.AwsRegion, s.config.CognitoUserPoolId)

	resp, err := http.Get(url)
	if err != nil {
		logging.Error("Failed to load cognito public key", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var jwks jwks
	json.Unmarshal(body, &jwks)

	for _, key := range jwks.Keys {
		// Decode Base64URL (without padding) `n` and `e`
		nBytes, _ := base64.RawURLEncoding.DecodeString(key.N)
		eBytes, _ := base64.RawURLEncoding.DecodeString(key.E)

		// Convert to big.Int and integer
		n := new(big.Int).SetBytes(nBytes)
		e := int(new(big.Int).SetBytes(eBytes).Int64())

		// Construct RSA Public Key
		s.cognitoPublicKeys[key.Kid] = &rsa.PublicKey{N: n, E: e}
	}
	logging.Info("cognito public key loaded")
}

// Validate JWT
func (s *server) validateJWT(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("invalid token: missing kid")
		}
		if key, found := s.cognitoPublicKeys[kid]; found {
			return key, nil
		}
		return nil, errors.New("invalid token: unknown kid")
	}, jwt.WithIssuer(fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", s.config.AwsRegion, s.config.CognitoUserPoolId)))
	if err != nil {
		return nil, err
	}
	return token, nil
}

func sha256Hash(payload []byte) string {
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:])
}

func signRequestWithSigV4(ctx context.Context, cfg aws.Config, req *http.Request) error {
	signer := v4.NewSigner()

	payload, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}
	req.Body = io.NopCloser(bytes.NewReader(payload)) // Reset body

	// Sign request
	credentials, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		logging.Error("Failed to save game", zap.Error(err))
	}
	err = signer.SignHTTP(ctx, credentials, req, sha256Hash(payload), "appsync", cfg.Region, time.Now())
	if err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	return nil
}
