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
)

var (
	dynamoClient           *dynamodb.Client
	ErrMatchRecordNotFound = fmt.Errorf("match record not found")
)

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	auth.MustAuth(event.RequestContext.Authorizer)
	matchId := event.PathParameters["id"]

	matchRecord, err := getMatchRecord(ctx, matchId)
	if err != nil {
		if errors.Is(err, ErrMatchRecordNotFound) {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound}, nil
		}
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError},
			fmt.Errorf("failed to get match record: %w", err)
	}

	matchRecordResp := dtos.MatchRecordGetResponseFromEntity(matchRecord)
	matchRecordJson, err := json.Marshal(matchRecordResp)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError},
			fmt.Errorf("failed to marshal response: %w", err)
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
	if matchRecordOutput.Item == nil {
		return entities.MatchRecord{}, ErrMatchRecordNotFound
	}
	var matchRecord entities.MatchRecord
	if err := attributevalue.UnmarshalMap(matchRecordOutput.Item, &matchRecord); err != nil {
		return entities.MatchRecord{}, err
	}
	return matchRecord, nil
}

func main() {
	lambda.Start(handler)
}
