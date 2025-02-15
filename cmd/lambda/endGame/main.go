package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var dynamoClient *dynamodb.Client

// Define event structure (if known)
type EndGameEvent struct {
	MatchId string `json:"matchId"`
	Player1 string `json:"player1"`
	Player2 string `json:"player2"`
}

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, event json.RawMessage) {
	// Or use a struct
	var endGameEvent EndGameEvent
	json.Unmarshal(event, &endGameEvent)

	_, err := dynamoClient.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
		TableName: aws.String("ActiveMatches"),
		Key: map[string]types.AttributeValue{
			"MatchId": &types.AttributeValueMemberS{Value: endGameEvent.MatchId},
		},
	})
	if err != nil {
		log.Fatalf("Failed to handle end game: %v", err)
	}

	_, err = dynamoClient.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
		TableName: aws.String("UserMatches"),
		Key: map[string]types.AttributeValue{
			"UserHandler": &types.AttributeValueMemberS{Value: endGameEvent.Player1},
		},
	})
	if err != nil {
		log.Fatalf("Failed to handle end game: %v", err)
	}

	_, err = dynamoClient.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
		TableName: aws.String("UserMatches"),
		Key: map[string]types.AttributeValue{
			"UserHandler": &types.AttributeValueMemberS{Value: endGameEvent.Player2},
		},
	})
	if err != nil {
		log.Fatalf("Failed to handle end game: %v", err)
	}
}

func main() {
	lambda.Start(handler)
}
