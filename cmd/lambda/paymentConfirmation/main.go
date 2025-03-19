package main

import (
	"context"
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
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
)

var (
	webhookSecret = os.Getenv("STRIPE_WEBHOOK_SECRET")
	storageClient *storage.Client
)

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}
	storageClient = storage.NewClient(dynamodb.NewFromConfig(cfg))
}

// Handle Stripe Webhook
func handler(
	ctx context.Context,
	request events.APIGatewayProxyRequest,
) (
	events.APIGatewayProxyResponse,
	error,
) {
	event := stripe.Event{}

	// Verify the webhook signature
	sigHeader := request.Headers["Stripe-Signature"]
	body := []byte(request.Body)

	event, err := webhook.ConstructEvent(body, sigHeader, webhookSecret)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
		}, fmt.Errorf("failed to verfiy webhook signature: %w", err)
	}

	// Process event
	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		err := json.Unmarshal(event.Data.Raw, &session)
		if err != nil {
			log.Printf("Error parsing session data: %v", err)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
			}, fmt.Errorf("failed to parse session data: %w", err)
		}

		userId := session.ClientReferenceID
		err = storageClient.UpdateUserProfile(
			ctx,
			userId,
			storage.UserProfileUpdateOptions{
				Membership: aws.String("membership"),
			},
		)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
			}, fmt.Errorf("failed to update user profile: %w", err)
		}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
	}, nil
}

func main() {
	lambda.Start(handler)
}
