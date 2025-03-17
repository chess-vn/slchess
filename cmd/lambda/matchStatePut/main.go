package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/chess-vn/slchess/internal/aws/storage"
	"github.com/chess-vn/slchess/internal/domains/dtos"
)

var storageClient *storage.Client

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	storageClient = storage.NewClient(dynamodb.NewFromConfig(cfg))
}

func handler(
	ctx context.Context,
	event map[string]interface{},
) (
	map[string]interface{},
	error,
) {
	arguments := event["arguments"].(map[string]interface{})
	reqJson, _ := json.Marshal(arguments["input"])

	var matchStateReq dtos.MatchStateRequest
	if err := json.Unmarshal(reqJson, &matchStateReq); err != nil {
		return nil, fmt.Errorf("failed to extract input: %w", err)
	}

	matchState := dtos.MatchStateRequestToEntity(matchStateReq)
	if err := storageClient.PutMatchState(ctx, matchState); err != nil {
		return nil, fmt.Errorf("failed to put match state: %w", err)
	}

	return map[string]interface{}{
		"Id":      matchState.Id,
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
