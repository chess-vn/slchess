package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

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

var dynamoClient *dynamodb.Client

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	auth.MustAuth(event.RequestContext.Authorizer)
	matchId := event.PathParameters["id"]
	startKey, limit, err := extractScanParameters(event.QueryStringParameters)
	if err != nil {
		logging.Error("Failed to get match states", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest}, nil
	}
	matchStates, lastEvaluatedKey, err := fetchMatchStates(ctx, matchId, startKey, limit)
	if err != nil {
		logging.Error("Failed to get match states", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	matchStateListResp := dtos.MatchStateListResponseFromEntities(matchStates)
	if lastEvaluatedKey != nil {
		matchStateListResp.NextPageToken = dtos.NextMatchStatePageToken{
			Timestamp: lastEvaluatedKey["Timestamp"].(*types.AttributeValueMemberS).Value,
		}
	}

	matchStateListJson, err := json.Marshal(matchStateListResp)
	if err != nil {
		logging.Error("Failed to get match states", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}
	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(matchStateListJson)}, nil
}

func fetchMatchStates(ctx context.Context, matchId string, lastKey map[string]types.AttributeValue, limit int32) ([]entities.MatchState, map[string]types.AttributeValue, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String("MatchStates"),
		KeyConditionExpression: aws.String("MatchId = :matchId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":matchId": &types.AttributeValueMemberS{Value: matchId},
		},
		ScanIndexForward: aws.Bool(false), // Sort by timestamp DESCENDING (most recent first)
		Limit:            aws.Int32(limit),
	}
	if lastKey != nil {
		input.ExclusiveStartKey = lastKey
	}
	matchStatesOutput, err := dynamoClient.Query(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var matchStates []entities.MatchState
	if err := attributevalue.UnmarshalListOfMaps(matchStatesOutput.Items, &matchStates); err != nil {
		return nil, nil, err
	}

	return matchStates, matchStatesOutput.LastEvaluatedKey, nil
}

func extractScanParameters(params map[string]string) (map[string]types.AttributeValue, int32, error) {
	var limit int32
	if limitStr, ok := params["limit"]; ok {
		limitInt64, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid limit: %v", err)
		}
		limit = int32(limitInt64)
	} else {
		limit = 20
	}

	// Check for startKey (optional)
	var startKey map[string]types.AttributeValue
	if startKeyStr, ok := params["startKey"]; ok {
		startKey = map[string]types.AttributeValue{
			"Timestamp": &types.AttributeValueMemberS{Value: startKeyStr},
		}
	}

	return startKey, int32(limit), nil
}

func main() {
	lambda.Start(handler)
}
