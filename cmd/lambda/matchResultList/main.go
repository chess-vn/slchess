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
	userId := mustAuth(event.RequestContext.Authorizer)
	startKey, limit, err := extractScanParameters(userId, event.PathParameters)
	if err != nil {
		logging.Error("Failed to get match record", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest}, nil
	}
	matchResults, lastEvaluatedKey, err := fetchMatchResults(ctx, userId, startKey, limit)
	if err != nil {
		logging.Error("Failed to get match record", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	matchResultListResp := dtos.MatchResultListResponseFromEntities(matchResults)
	if lastEvaluatedKey != nil {
		matchResultListResp.NextPageToken = dtos.NextPageToken{
			MatchId: lastEvaluatedKey["MatchId"].(*types.AttributeValueMemberS).Value,
		}
	}

	matchResultListJson, err := json.Marshal(matchResultListResp)
	if err != nil {
		logging.Error("Failed to get match record", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}
	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(matchResultListJson)}, nil
}

func fetchMatchResults(ctx context.Context, userId string, lastKey map[string]types.AttributeValue, limit int32) ([]entities.MatchResult, map[string]types.AttributeValue, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String("MatchResults"),
		KeyConditionExpression: aws.String("UserId = :userId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userId": &types.AttributeValueMemberS{Value: userId},
		},
		ScanIndexForward: aws.Bool(false), // Sort by timestamp DESCENDING (most recent first)
		Limit:            aws.Int32(limit),
	}
	if lastKey != nil {
		input.ExclusiveStartKey = lastKey
	}
	matchResultOutputs, err := dynamoClient.Query(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var matchResults []entities.MatchResult
	if err := attributevalue.UnmarshalListOfMaps(matchResultOutputs.Items, &matchResults); err != nil {
		return nil, nil, err
	}

	return matchResults, matchResultOutputs.LastEvaluatedKey, nil
}

func mustAuth(authorizer map[string]interface{}) string {
	v, exists := authorizer["claims"]
	if !exists {
		panic("no authorizer claims")
	}
	claims, ok := v.(map[string]interface{})
	if !ok {
		panic("claims must be of type map")
	}
	userId, ok := claims["sub"].(string)
	if !ok {
		panic("invalid sub")
	}
	return userId
}

func extractScanParameters(userId string, params map[string]string) (map[string]types.AttributeValue, int32, error) {
	limitStr, ok := params["limit"]
	if !ok {
		return nil, 0, fmt.Errorf("missing parameter: limit")
	}

	limit, err := strconv.ParseInt(limitStr, 10, 32)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid limit: %v", err)
	}

	// Check for startKey (optional)
	startKey := make(map[string]types.AttributeValue)
	if startKeyStr, ok := params["startKey"]; ok {
		startKey = map[string]types.AttributeValue{
			"UserId":  &types.AttributeValueMemberS{Value: userId},
			"MatchId": &types.AttributeValueMemberS{Value: startKeyStr},
		}
	}

	return startKey, int32(limit), nil
}

func main() {
	lambda.Start(handler)
}
