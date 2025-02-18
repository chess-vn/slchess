package main

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/chess-vn/slchess/internal/domains/entities"
	"github.com/chess-vn/slchess/pkg/logging"
	"go.uber.org/zap"
)

var dynamoClient *dynamodb.Client

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, event json.RawMessage) {
	// Get match data
	var matchRecord entities.MatchRecord
	if err := json.Unmarshal(event, &matchRecord); err != nil {
		logging.Fatal("Failed to handle end game: %v", zap.Error(err))
	}

	_, err := dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("ActiveMatches"),
		Key: map[string]types.AttributeValue{
			"MatchId": &types.AttributeValueMemberS{Value: matchRecord.MatchId},
		},
	})
	if err != nil {
		logging.Fatal("Failed to handle end game: %v", zap.Error(err))
	}

	_, err = dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("UserMatches"),
		Key: map[string]types.AttributeValue{
			"UserHandler": &types.AttributeValueMemberS{Value: matchRecord.Players[0].Handler},
		},
	})
	if err != nil {
		logging.Fatal("Failed to handle end game: %v", zap.Error(err))
	}

	_, err = dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("UserMatches"),
		Key: map[string]types.AttributeValue{
			"UserHandler": &types.AttributeValueMemberS{Value: matchRecord.Players[1].Handler},
		},
	})
	if err != nil {
		logging.Fatal("Failed to handle end game: %v", zap.Error(err))
	}

	av, err := attributevalue.MarshalMap(matchRecord)
	if err != nil {
		logging.Fatal("Failed to handle end game: %v", zap.Error(err))
	}
	dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("MatchRecords"),
		Item:      av,
	})

	// TODO: calculate new rating for both players
}

func main() {
	lambda.Start(handler)
}
