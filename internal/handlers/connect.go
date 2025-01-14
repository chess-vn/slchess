package handlers

import (
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

func ConnectHandler(request events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	sess := session.Must(session.NewSession())
	svc := dynamodb.New(sess)

	connectionID := request.RequestContext.ConnectionID
	_, err := svc.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String("WebSocketConnections"),
		Item: map[string]*dynamodb.AttributeValue{
			"ConnectionID": {S: aws.String(connectionID)},
		},
	})
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500}, fmt.Errorf("failed to save connection: %w", err)
	}

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}
