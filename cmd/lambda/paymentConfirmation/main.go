package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/chess-vn/slchess/internal/aws/storage"
)

var (
	storageClient *storage.Client
	key2          = os.Getenv("ZALOPAY_KEY2") // set in Lambda environment variable
)

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}
	storageClient = storage.NewClient(dynamodb.NewFromConfig(cfg))
}

type CallbackBody struct {
	Data string `json:"data"`
	Mac  string `json:"mac"`
}

func handler(
	ctx context.Context,
	request events.APIGatewayProxyRequest,
) (
	events.APIGatewayProxyResponse,
	error,
) {
	var body CallbackBody
	if err := json.Unmarshal([]byte(request.Body), &body); err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
		}, fmt.Errorf("invalid JSON: %w", err)
	}

	// Compute HMAC SHA256
	h := hmac.New(sha256.New, []byte(key2))
	h.Write([]byte(body.Data))
	expectedMac := hex.EncodeToString(h.Sum(nil))

	if expectedMac != body.Mac {
		log.Println("MAC mismatch")
		resp := map[string]interface{}{
			"return_code":    -1,
			"return_message": "mac not equal",
		}
		respBytes, _ := json.Marshal(resp)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Body:       string(respBytes),
		}, nil
	}

	// Valid callback
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(body.Data), &data); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest}, fmt.Errorf("invalid data payload: %w", err)
	}

	appTransID := data["app_trans_id"]
	userId := data["app_user"]
	// membership := data["item"].([]interface{})[0].(map[string]interface{})["itename"].(string)
	membership := "pro"

	log.Printf("Received valid payment for user %s with plan %s (app_trans_id = %v)", userId, membership, appTransID)

	// Update user profile
	err := storageClient.UpdateUserProfile(ctx, userId.(string), storage.UserProfileUpdateOptions{
		Membership: aws.String(membership),
	})
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to update user profile: %w", err)
	}

	// Respond to ZaloPay
	resp := map[string]interface{}{
		"return_code":    1,
		"return_message": "success",
	}
	respBytes, _ := json.Marshal(resp)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(respBytes),
	}, nil
}

func main() {
	lambda.Start(handler)
}
