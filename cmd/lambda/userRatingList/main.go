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
	startKey, limit, err := extractScanParameters(event.QueryStringParameters)
	if err != nil {
		logging.Error("Failed to list user ratings", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest}, nil
	}
	userRatings, lastEvaluatedKey, err := fetchUserRatingList(ctx, startKey, limit)
	if err != nil {
		logging.Error("Failed to list user ratings", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	userRatingListResp := dtos.UserRatingListResponseFromEntities(userRatings)
	if lastEvaluatedKey != nil {
		userRatingListResp.NextPageToken = dtos.NextUserRatingPageToken{
			Rating: lastEvaluatedKey["Rating"].(*types.AttributeValueMemberS).Value,
		}
	}

	userRatingListJson, err := json.Marshal(userRatingListResp)
	if err != nil {
		logging.Error("Failed to list user ratings", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}
	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(userRatingListJson)}, nil
}

func fetchUserRatingList(ctx context.Context, lastKey map[string]types.AttributeValue, limit int32) ([]entities.UserRating, map[string]types.AttributeValue, error) {
	input := &dynamodb.QueryInput{
		TableName:        aws.String("UserRatings"),
		IndexName:        aws.String("RatingIndex"),
		ScanIndexForward: aws.Bool(false),
		Limit:            aws.Int32(limit),
	}
	if lastKey != nil {
		input.ExclusiveStartKey = lastKey
	}
	userRatingsOutput, err := dynamoClient.Query(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var userRatings []entities.UserRating
	if err := attributevalue.UnmarshalListOfMaps(userRatingsOutput.Items, &userRatings); err != nil {
		return nil, nil, err
	}

	return userRatings, userRatingsOutput.LastEvaluatedKey, nil
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

func extractScanParameters(params map[string]string) (map[string]types.AttributeValue, int32, error) {
	limitStr, ok := params["limit"]
	if !ok {
		return nil, 0, fmt.Errorf("missing parameter: limit")
	}

	limit, err := strconv.ParseInt(limitStr, 10, 32)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid limit: %v", err)
	}

	// Check for startKey (optional)
	var startKey map[string]types.AttributeValue
	if startKeyStr, ok := params["startKey"]; ok {
		startKey = map[string]types.AttributeValue{
			"Rating": &types.AttributeValueMemberS{Value: startKeyStr},
		}
	}

	return startKey, int32(limit), nil
}

func main() {
	lambda.Start(handler)
}
