package main

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/golang-jwt/jwt/v5"
)

var (
	dynamoClient      *dynamodb.Client
	cognitoPublicKeys map[string]*rsa.PublicKey
)

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
	loadCognitoPublicKeys()
}

// Struct for Cognito's JWKS JSON response
type jwk struct {
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwks struct {
	Keys []jwk `json:"keys"`
}

// Load Cognito public keys
func loadCognitoPublicKeys() {
	userPoolId := os.Getenv("COGNITO_USER_POOL_ID")
	region := os.Getenv("AWS_REGION")
	url := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json", region, userPoolId)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error fetching Cognito public keys:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var jwks jwks
	json.Unmarshal(body, &jwks)

	cognitoPublicKeys = make(map[string]*rsa.PublicKey)
	for _, key := range jwks.Keys {
		// Decode Base64URL (without padding) `n` and `e`
		nBytes, _ := decodeBase64URL(key.N)
		eBytes, _ := decodeBase64URL(key.E)

		// Convert to big.Int and integer
		n := new(big.Int).SetBytes(nBytes)
		e := int(new(big.Int).SetBytes(eBytes).Int64())

		// Construct RSA Public Key
		cognitoPublicKeys[key.Kid] = &rsa.PublicKey{N: n, E: e}
		fmt.Println(key)
	}
}

// Decode Base64URL without padding
func decodeBase64URL(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// Validate JWT
func validateJWT(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("invalid token: missing kid")
		}
		if key, found := cognitoPublicKeys[kid]; found {
			return key, nil
		}
		return nil, errors.New("invalid token: unknown kid")
	}, jwt.WithIssuer(fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", os.Getenv("AWS_REGION"), os.Getenv("COGNITO_USER_POOL_ID"))))
	if err != nil {
		return nil, err
	}
	return token, nil
}

// Handle WebSocket connection with authentication
func handler(event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionId := event.RequestContext.ConnectionID
	token := event.Headers["Authorization"]

	if token == "" {
		return events.APIGatewayProxyResponse{StatusCode: 401, Body: "Unauthorized: No token provided"}, nil
	}

	validToken, err := validateJWT(token)
	if err != nil || !validToken.Valid {
		return events.APIGatewayProxyResponse{StatusCode: 401, Body: fmt.Sprintf("Unauthorized: %v", err)}, nil
	}

	userId := validToken.Claims.(jwt.MapClaims)["sub"].(string)

	// Store connection in DynamoDB
	_, err = dynamoClient.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String("Connections"),
		Item: map[string]types.AttributeValue{
			"Id":     &types.AttributeValueMemberS{Value: connectionId},
			"UserId": &types.AttributeValueMemberS{Value: userId},
		},
	})
	if err != nil {
		fmt.Println("Error saving connection:", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Internal Server Error"}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: 200, Body: "Connected and authenticated"}, nil
}

func main() {
	lambda.Start(handler)
}
