package server

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"

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
