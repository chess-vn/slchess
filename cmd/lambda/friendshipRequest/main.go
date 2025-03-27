package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/chess-vn/slchess/internal/aws/auth"
	"github.com/chess-vn/slchess/internal/aws/notification"
	"github.com/chess-vn/slchess/internal/aws/storage"
	"github.com/chess-vn/slchess/internal/domains/entities"
)

var (
	storageClient *storage.Client
	notiClient    *notification.Client
)

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	storageClient = storage.NewClient(dynamodb.NewFromConfig(cfg))
	notiClient = notification.NewClient(sns.NewFromConfig(cfg))
}

func handler(
	ctx context.Context,
	event events.APIGatewayProxyRequest,
) (
	events.APIGatewayProxyResponse,
	error,
) {
	userId := auth.MustAuth(event.RequestContext.Authorizer)
	targetId := event.PathParameters["id"]

	friendship := entities.Friendship{
		UserId:   userId,
		FriendId: targetId,
		Status:   "pending",
	}
	err := storageClient.PutFriendship(ctx, friendship)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to put friendship: %w", err)
	}

	// TODO: notify other user
	//
	// err = notiClient.SendPushNotification(ctx, "", "")
	// if err != nil {
	// 	return events.APIGatewayProxyResponse{
	// 		StatusCode: http.StatusInternalServerError,
	// 	}, fmt.Errorf("failed to send push notification: %w", err)
	// }

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
	}, nil
}

func main() {
	lambda.Start(handler)
}
