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
)

var dynamoClient *dynamodb.Client

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	userId := auth.MustAuth(event.RequestContext.Authorizer)
	targetId, startKey, limit, err := extractScanParameters(userId, event.QueryStringParameters)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest},
			fmt.Errorf("failed to extract parameters: %w", err)
	}
	matchResults, lastEvaluatedKey, err := fetchMatchResults(ctx, targetId, startKey, limit)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError},
			fmt.Errorf("failed to fetch match results: %w", err)
	}

	matchResultListResp := dtos.MatchResultListResponseFromEntities(matchResults)
	if lastEvaluatedKey != nil {
		matchResultListResp.NextPageToken = dtos.NextMatchResultPageToken{
			Timestamp: lastEvaluatedKey["Timestamp"].(*types.AttributeValueMemberS).Value,
		}
	}

	matchResultListJson, err := json.Marshal(matchResultListResp)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError},
			fmt.Errorf("failed to marshal response: %w", err)
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
	matchResultsOutput, err := dynamoClient.Query(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var matchResults []entities.MatchResult
	if err := attributevalue.UnmarshalListOfMaps(matchResultsOutput.Items, &matchResults); err != nil {
		return nil, nil, err
	}

	return matchResults, matchResultsOutput.LastEvaluatedKey, nil
}

func extractScanParameters(userId string, params map[string]string) (string, map[string]types.AttributeValue, int32, error) {
	var targetId string
	if userIdStr, ok := params["userId"]; ok {
		targetId = userId
	} else {
		targetId = userIdStr
	}

	var limit int32
	if limitStr, ok := params["limit"]; ok {
		limitInt64, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil {
			return "", nil, 0, fmt.Errorf("invalid limit: %v", err)
		}
		limit = int32(limitInt64)
	} else {
		limit = 10
	}

	// Check for startKey (optional)
	var startKey map[string]types.AttributeValue
	if startKeyStr, ok := params["startKey"]; ok {
		startKey = map[string]types.AttributeValue{
			"UserId":    &types.AttributeValueMemberS{Value: userId},
			"Timestamp": &types.AttributeValueMemberS{Value: startKeyStr},
		}
	}

	return targetId, startKey, int32(limit), nil
}

func main() {
	lambda.Start(handler)
}
