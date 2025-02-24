package main

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/chess-vn/slchess/internal/domains/dtos"
	"github.com/chess-vn/slchess/pkg/logging"
	"go.uber.org/zap"
)

var dynamoClient *dynamodb.Client

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, event json.RawMessage) {
	var matchStateReq dtos.MatchStateRequest
	if err := json.Unmarshal(event, &matchStateReq); err != nil {
		logging.Fatal("Failed to save match state", zap.Error(err))
	}

	matchState := dtos.MatchStateRequestToEntity(matchStateReq)
	av, err := attributevalue.MarshalMap(matchState)
	if err != nil {
		logging.Fatal("Failed to save match state", zap.Error(err))
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("MatchStates"),
		Item:      av,
	})
	if err != nil {
		logging.Fatal("Failed to save match state", zap.Error(err))
	}
}

func main() {
	lambda.Start(handler)
}
