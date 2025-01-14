package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

func MakeMoveHandler(request events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Parse incoming move
	var moveRequest struct {
		GameID string `json:"GameID"`
		Move   string `json:"Move"`
	}
	json.Unmarshal([]byte(request.Body), &moveRequest)

	sess := session.Must(session.NewSession())

	// Fetch game state from DynamoDB (add validation logic here)
	svc := dynamodb.New(sess)
	output, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String("GameStates"),
		Key: map[string]*dynamodb.AttributeValue{
			"GameID": {S: aws.String(moveRequest.GameID)},
		},
	})
	var game GameState
	err = dynamodbattribute.UnmarshalMap(output.Item, &game)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500}, fmt.Errorf("failed to retrieve game state: %w", err)
	}

	// Update the game state and notify the opponent

	// Send message to opponent
	apigateway := apigatewaymanagementapi.New(
		sess,
		aws.NewConfig().WithEndpoint(""),
	)

	message := fmt.Sprintf("Move made: %s", moveRequest.Move)
	_, err = apigateway.PostToConnection(&apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String("opponent-connection-id"),
		Data:         []byte(message),
	})
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500}, fmt.Errorf("failed to send message: %w", err)
	}

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}
