package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/chess-vn/slchess/internal/domains/entities"
)

var (
	apiGatewayClient *apigatewaymanagementapi.Client
	dynamoClient     *dynamodb.Client
	ctx              context.Context

	region            = os.Getenv("AWS_REGION")
	websocketApiId    = os.Getenv("WEBSOCKET_API_ID")
	websocketApiStage = os.Getenv("WEBSOCKET_API_STAGE")
)

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
	apiEndpoint := fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/%s", websocketApiId, region, websocketApiStage)
	apiGatewayClient = apigatewaymanagementapi.New(apigatewaymanagementapi.Options{
		BaseEndpoint: aws.String(apiEndpoint),
		Region:       region,
		Credentials:  cfg.Credentials,
	})
}

func handler(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionId := event.RequestContext.ConnectionID

	// Get user ID from DynamoDB
	connectionOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("Connections"),
		Key: map[string]types.AttributeValue{
			"Id": &types.AttributeValueMemberS{Value: connectionId},
		},
	})
	if err != nil || connectionOutput.Item == nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized}, nil
	}
	var connection entities.Connection
	if err := attributevalue.UnmarshalMap(connectionOutput.Item, &connection); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	activeMatch, exist, err := checkForActiveMatch(ctx, connection.UserId)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}
	if exist {
		activeMatchJson, err := json.Marshal(activeMatch)
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
		}
		_, err = apiGatewayClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: &connection.Id,
			Data:         activeMatchJson,
		})
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to send message"}, nil
		}
		return events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
}

func checkForActiveMatch(ctx context.Context, userId string) (entities.ActiveMatch, bool, error) {
	userMatchOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("UserMatches"),
		Key: map[string]types.AttributeValue{
			"UserId": &types.AttributeValueMemberS{
				Value: userId,
			},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return entities.ActiveMatch{}, false, err
	}
	if userMatchOutput.Item == nil {
		return entities.ActiveMatch{}, false, nil
	}

	var userMatch entities.UserMatch
	if err := attributevalue.UnmarshalMap(userMatchOutput.Item, &userMatch); err != nil {
		return entities.ActiveMatch{}, false, err
	}

	activeMatchOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("ActiveMatches"),
		Key: map[string]types.AttributeValue{
			"MatchId": &types.AttributeValueMemberS{Value: userMatch.MatchId},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return entities.ActiveMatch{}, false, err
	}
	if activeMatchOutput.Item == nil {
		return entities.ActiveMatch{}, false, nil
	}

	var activeMatch entities.ActiveMatch
	if err := attributevalue.UnmarshalMap(userMatchOutput.Item, activeMatch); err != nil {
		return entities.ActiveMatch{}, false, err
	}
	return activeMatch, true, nil
}

func main() {
	lambda.Start(handler)
}
