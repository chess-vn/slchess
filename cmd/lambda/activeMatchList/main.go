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
	mustAuth(event.RequestContext.Authorizer)
	gameMode, startKey, limit, err := extractParameters(event.QueryStringParameters)
	if err != nil {
		logging.Error("Failed to list active matches", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest}, nil
	}
	activeMatches, lastEvaluatedKey, err := fetchActiveMatchList(ctx, gameMode, startKey, limit)
	if err != nil {
		logging.Error("Failed to list active matches", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	activeMatchListResp := dtos.ActiveMatchListResponseFromEntities(activeMatches)
	if lastEvaluatedKey != nil {
		activeMatchListResp.NextPageToken = dtos.NextActiveMatchPageToken{
			CreatedAt: lastEvaluatedKey["CreatedAt"].(*types.AttributeValueMemberS).Value,
		}
	}

	matchResultListJson, err := json.Marshal(activeMatchListResp)
	if err != nil {
		logging.Error("Failed to list active matches", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}
	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(matchResultListJson)}, nil
}

func fetchActiveMatchList(ctx context.Context, lastKey map[string]types.AttributeValue, limit int32) ([]entities.ActiveMatch, map[string]types.AttributeValue, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String("ActiveMatches"),
		IndexName:              aws.String("AverageRatingIndex"),
		KeyConditionExpression: aws.String("#pk = :pk AND #rating >= :rating"),
		ExpressionAttributeNames: map[string]string{
			"#pk":     "PartitionKey",
			"#rating": "AverageRating",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":rating":   &types.AttributeValueMemberN{Value: "1600.0"},
			":gameMode": &types.AttributeValueMemberS{Value: gameMode},
		},
		ScanIndexForward: aws.Bool(false),
		Limit:            aws.Int32(limit),
	}
	if gameMode == "" {
		input.FilterExpression = aws.String("AverageRating >= :rating")
		delete(input.ExpressionAttributeValues, ":gameMode")
	}
	if lastKey != nil {
		input.ExclusiveStartKey = lastKey
	}
	activeMatchesOutput, err := dynamoClient.Query(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var activeMatches []entities.ActiveMatch
	if err := attributevalue.UnmarshalListOfMaps(activeMatchesOutput.Items, &activeMatches); err != nil {
		return nil, nil, err
	}

	return activeMatches, activeMatchesOutput.LastEvaluatedKey, nil
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

func extractParameters(params map[string]string) (string, map[string]types.AttributeValue, int32, error) {
	gameMode := params["gameMode"]

	limitStr, ok := params["limit"]
	if !ok {
		return "", nil, 0, fmt.Errorf("missing parameter: limit")
	}

	limit, err := strconv.ParseInt(limitStr, 10, 32)
	if err != nil {
		return "", nil, 0, fmt.Errorf("invalid limit: %v", err)
	}

	// Check for startKey (optional)
	var startKey map[string]types.AttributeValue
	if startKeyStr, ok := params["startKey"]; ok {
		startKey = map[string]types.AttributeValue{
			"CreatedAt": &types.AttributeValueMemberS{Value: startKeyStr},
		}
	}

	return gameMode, startKey, int32(limit), nil
}

func main() {
	lambda.Start(handler)
}
