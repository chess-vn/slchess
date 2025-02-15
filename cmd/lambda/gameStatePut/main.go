package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/bucket-sort/slchess/internal/domains/entities"
)

var dynamoClient *dynamodb.Client

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, event json.RawMessage) {
	var matchState entities.MatchState
	json.Unmarshal(event, &matchState)

	av, err := attributevalue.MarshalMap(matchState)
	if err != nil {
		log.Fatalf("Failed to save match state: %v", err)
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("MatchStates"),
		Item:      av,
	})
	if err != nil {
		log.Fatalf("Failed to save match state: %v", err)
	}
}

func main() {
	lambda.Start(handler)
}
