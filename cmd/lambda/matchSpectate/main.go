package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/chess-vn/slchess/internal/aws/auth"
	"github.com/chess-vn/slchess/internal/domains/dtos"
	"github.com/chess-vn/slchess/internal/domains/entities"
	"github.com/chess-vn/slchess/pkg/logging"
	"go.uber.org/zap"
)

var (
	dynamoClient                     *dynamodb.Client
	ErrMatchStateNotFound            = fmt.Errorf("match state not found")
	ErrSpectatorConversationNotFound = fmt.Errorf("spectator conversation not found")
)

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	auth.MustAuth(event.RequestContext.Authorizer)
	matchId := event.PathParameters["id"]

	matchState, err := getMatchState(ctx, matchId)
	if err != nil {
		if !errors.Is(err, ErrMatchStateNotFound) {
			logging.Error("Failed to get match state", zap.Error(err))
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
		}
	}

	spectatorConversation, err := getSpectatorConversation(ctx, matchId)
	if err != nil {
		logging.Error("Failed to get spectator conversation", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	resp := dtos.NewMatchSpectateResponse(matchState, spectatorConversation.ConversationId)
	respJson, err := json.Marshal(resp)
	if err != nil {
		logging.Error("Failed to get match state", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(respJson)}, nil
}

func getMatchState(ctx context.Context, matchId string) (entities.MatchState, error) {
	matchRecordOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("MatchStates"),
		Key: map[string]types.AttributeValue{
			"MatchId": &types.AttributeValueMemberS{
				Value: matchId,
			},
		},
	})
	if err != nil {
		return entities.MatchState{}, err
	}
	if matchRecordOutput.Item == nil {
		return entities.MatchState{}, ErrMatchStateNotFound
	}
	var matchState entities.MatchState
	if err := attributevalue.UnmarshalMap(matchRecordOutput.Item, &matchState); err != nil {
		return entities.MatchState{}, err
	}
	return matchState, nil
}

func getSpectatorConversation(ctx context.Context, matchId string) (entities.SpectatorConversation, error) {
	spectatorConversationOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("SpectatorConversations"),
		Key: map[string]types.AttributeValue{
			"MatchId": &types.AttributeValueMemberS{
				Value: matchId,
			},
		},
	})
	if err != nil {
		return entities.SpectatorConversation{}, err
	}
	if spectatorConversationOutput.Item == nil {
		return entities.SpectatorConversation{}, ErrSpectatorConversationNotFound
	}
	var spectatorConversation entities.SpectatorConversation
	if err := attributevalue.UnmarshalMap(spectatorConversationOutput.Item, &spectatorConversation); err != nil {
		return entities.SpectatorConversation{}, err
	}
	return spectatorConversation, nil
}

func main() {
	lambda.Start(handler)
}
