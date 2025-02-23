package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var dynamoClient *dynamodb.Client

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
}

func handler(request events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionId := request.RequestContext.ConnectionID
	_, err := dynamoClient.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
		TableName: aws.String("Connections"),
		Key: map[string]types.AttributeValue{
			"Id": &types.AttributeValueMemberS{Value: connectionId},
		},
	})
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500}, fmt.Errorf("failed to delete connecction: %w", err)
	}

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

func main() {
	lambda.Start(handler)
}
