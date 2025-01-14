package handlers

import (
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

func DisconnectHandler(request events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	sess := session.Must(session.NewSession())
	svc := dynamodb.New(sess)

	connectionID := request.RequestContext.ConnectionID
	_, err := svc.DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String("WebSocketConnections"),
		Key: map[string]*dynamodb.AttributeValue{
			"ConnectionID": {S: aws.String(connectionID)},
		},
	})
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500}, fmt.Errorf("failed to delete connecction: %w", err)
	}

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}
