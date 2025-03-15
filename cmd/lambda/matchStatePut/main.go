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

func handler(ctx context.Context, event map[string]interface{}) (map[string]interface{}, error) {
	var matchStateReq dtos.MatchStateRequest
	reqJson, _ := json.Marshal(event["arguments"].(map[string]interface{})["input"])
	if err := json.Unmarshal(reqJson, &matchStateReq); err != nil {
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

	return map[string]interface{}{
		"MatchId": matchState.MatchId,
		"PlayerStates": []map[string]interface{}{
			{
				"Clock":  matchState.PlayerStates[0].Clock,
				"Status": matchState.PlayerStates[0].Status,
			},
			{
				"Clock":  matchState.PlayerStates[1].Clock,
				"Status": matchState.PlayerStates[1].Status,
			},
		},
		"Move": map[string]interface{}{
			"PlayerId": matchState.Move.PlayerId,
			"Uci":      matchState.Move.Uci,
		},
		"GameState": matchState.GameState,
		"Ply":       matchState.Ply,
		"Timestamp": matchState.Timestamp,
	}, nil
}

func main() {
	lambda.Start(handler)
}
