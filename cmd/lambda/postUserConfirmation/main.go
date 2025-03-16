package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/chess-vn/slchess/internal/domains/entities"
)

var dynamoClient *dynamodb.Client

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, event events.CognitoEventUserPoolsPostConfirmation) (events.CognitoEventUserPoolsPostConfirmation, error) {
	userId := event.Request.UserAttributes["sub"]
	username := event.UserName

	// Default user profile
	userProfile := entities.UserProfile{
		UserId:     userId,
		Username:   username,
		Membership: "guest",
		CreatedAt:  time.Now(),
	}
	userProfileAv, err := attributevalue.MarshalMap(userProfile)
	if err != nil {
		return event, fmt.Errorf("failed to marshal user profile map: %w", err)
	}
	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("UserProfiles"),
		Item:      userProfileAv,
	})
	if err != nil {
		return event, fmt.Errorf("failed to put user profile: %w", err)
	}

	// Default user rating
	userRating := entities.UserRating{
		UserId:       userId,
		Rating:       1200,
		RD:           200,
		PartitionKey: "UserRatings",
	}
	userRatingAv, err := attributevalue.MarshalMap(userRating)
	if err != nil {
		return event, fmt.Errorf("failed to marshal user rating map: %w", err)
	}
	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("UserRatings"),
		Item:      userRatingAv,
	})
	if err != nil {
		return event, fmt.Errorf("failed to put user rating: %w", err)
	}

	return event, nil
}

func main() {
	lambda.Start(handler)
}
