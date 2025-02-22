package main

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/chess-vn/slchess/internal/domains/entities"
)

var (
	dynamoClient *dynamodb.Client
	ctx          context.Context
)

// Handle matchmaking requests
func handler(event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionId := event.RequestContext.ConnectionID

	// Get user ID from DynamoDB
	connectionOutput, err := dynamoClient.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String("Connections"),
		Key: map[string]types.AttributeValue{
			"Id": &types.AttributeValueMemberS{Value: connectionId},
		},
	})
	if err != nil || connectionOutput.Item == nil {
		return events.APIGatewayProxyResponse{StatusCode: 401, Body: "Unauthorized"}, nil
	}

	userId := connectionOutput.Item["UserId"].(*types.AttributeValueMemberS).Value

	_, exist, err := checkForActiveMatch(ctx, userId)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}
	if exist {
		// TODO: Notify user about match info

		return events.APIGatewayProxyResponse{StatusCode: 200}, nil
	}

	// TODO: Wait here until client disconnect by themselve

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
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
