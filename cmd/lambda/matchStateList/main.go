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

const (
	ASC  = true
	DESC = false
)

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	auth.MustAuth(event.RequestContext.Authorizer)
	matchId := event.PathParameters["id"]
	startKey, limit, order, err := extractScanParameters(matchId, event.QueryStringParameters)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest},
			fmt.Errorf("failed to extract parameters: %w", err)
	}
	matchStates, lastEvaluatedKey, err := fetchMatchStates(ctx, matchId, startKey, limit, order)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError},
			fmt.Errorf("failed to fetch match states: %w", err)
	}
	fmt.Println(lastEvaluatedKey)

	matchStateListResp := dtos.MatchStateListResponseFromEntities(matchStates)
	if lastEvaluatedKey != nil {
		matchStateListResp.NextPageToken = &dtos.NextMatchStatePageToken{
			Id:  lastEvaluatedKey["Id"].(*types.AttributeValueMemberS).Value,
			Ply: lastEvaluatedKey["Ply"].(*types.AttributeValueMemberN).Value,
		}
	}

	matchStateListJson, err := json.Marshal(matchStateListResp)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError},
			fmt.Errorf("failed to marshal response: %w", err)
	}
	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(matchStateListJson)}, nil
}

func fetchMatchStates(ctx context.Context, matchId string, lastKey map[string]types.AttributeValue, limit int32, order bool) ([]entities.MatchState, map[string]types.AttributeValue, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String("MatchStates"),
		IndexName:              aws.String("MatchIndex"),
		KeyConditionExpression: aws.String("MatchId = :matchId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":matchId": &types.AttributeValueMemberS{Value: matchId},
		},
		ExclusiveStartKey: lastKey,
		ScanIndexForward:  aws.Bool(order), // Sort by timestamp DESCENDING (most recent first)
		Limit:             aws.Int32(limit),
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

func extractScanParameters(matchId string, params map[string]string) (map[string]types.AttributeValue, int32, bool, error) {
	var limit int32
	if limitStr, ok := params["limit"]; ok {
		limitInt64, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil {
			return nil, 0, false, fmt.Errorf("invalid limit: %v", err)
		}
		limit = int32(limitInt64)
	} else {
		limit = 20
	}

	// Check for startKey (optional)
	var startKey map[string]types.AttributeValue
	if startKeyStr, ok := params["startKey"]; ok {
		var nextPageToken dtos.NextMatchStatePageToken
		if err := json.Unmarshal([]byte(startKeyStr), &nextPageToken); err != nil {
			return nil, 0, false, err
		}
		startKey = map[string]types.AttributeValue{
			"Id":      &types.AttributeValueMemberS{Value: nextPageToken.Id},
			"MatchId": &types.AttributeValueMemberS{Value: matchId},
			"Ply":     &types.AttributeValueMemberN{Value: nextPageToken.Ply},
		}
	}

	var order bool
	if orderStr, ok := params["order"]; ok {
		if orderStr == "asc" {
			order = ASC
		}
	}

	return startKey, int32(limit), order, nil
}

func main() {
	lambda.Start(handler)
}
