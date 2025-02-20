package main

import (
	"context"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/chess-vn/slchess/internal/domains/entities"
	"github.com/chess-vn/slchess/pkg/logging"
	"go.uber.org/zap"
)

var dynamoClient *dynamodb.Client

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, event events.CognitoEventUserPoolsPostConfirmation) (events.CognitoEventUserPoolsPostConfirmation, error) {
	userId := event.Request.UserAttributes["sub"]

	// Default user profile
	userProfile := entities.UserProfile{
		UserId:     userId,
		Membership: "guest",
		CreatedAt:  time.Now(),
	}
	userProfileAv, err := attributevalue.MarshalMap(userProfile)
	if err != nil {
		logging.Fatal("Failed to save user rating", zap.Error(err))
	}
	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("UserProfiles"),
		Item:      userProfileAv,
	})
	if err != nil {
		logging.Fatal("Failed to save user rating", zap.Error(err))
	}

	// Default user rating
	userRating := entities.UserRating{
		UserId: userId,
		Rating: 1200,
		RD:     200,
	}
	userRatingAv, err := attributevalue.MarshalMap(userRating)
	if err != nil {
		logging.Fatal("Failed to save user rating", zap.Error(err))
	}
	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("UserRatings"),
		Item:      userRatingAv,
	})
	if err != nil {
		logging.Fatal("Failed to save user rating", zap.Error(err))
	}

	return event, nil
}

func main() {
	lambda.Start(handler)
}
