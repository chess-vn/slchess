package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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
	matchId := event.PathParameters["matchId"]

	matchRecord, err := getMatchRecord(ctx, matchId)
	if err != nil {
		logging.Error("Failed to get match record", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	matchRecordJson, err := json.Marshal(matchRecord)
	if err != nil {
		logging.Error("Failed to get match record", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}
	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(matchRecordJson)}, nil
}

func getMatchRecord(ctx context.Context, matchId string) (entities.MatchRecord, error) {
	matchRecordOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("MatchRecords"),
		Key: map[string]types.AttributeValue{
			"MatchId": &types.AttributeValueMemberS{
				Value: matchId,
			},
		},
	})
	if err != nil {
		return entities.MatchRecord{}, err
	}
	var matchRecord entities.MatchRecord
	if err := attributevalue.UnmarshalMap(matchRecordOutput.Item, &matchRecord); err != nil {
		return entities.MatchRecord{}, err
	}
	return matchRecord, nil
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

func main() {
	lambda.Start(handler)
}
