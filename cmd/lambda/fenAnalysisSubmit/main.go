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
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/chess-vn/slchess/internal/aws/analysis"
	"github.com/chess-vn/slchess/internal/domains/dtos"
)

var (
	apigatewayClient *apigatewaymanagementapi.Client
	analysisClient   *analysis.Client

	region            = os.Getenv("AWS_REGION")
	websocketApiId    = os.Getenv("WEBSOCKET_API_ID")
	websocketApiStage = os.Getenv("WEBSOCKET_API_STAGE")
)

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	apiEndpoint := fmt.Sprintf(
		"https://%s.execute-api.%s.amazonaws.com/%s",
		websocketApiId,
		region,
		websocketApiStage,
	)
	apigatewayClient = apigatewaymanagementapi.New(apigatewaymanagementapi.Options{
		BaseEndpoint: aws.String(apiEndpoint),
		Region:       region,
		Credentials:  cfg.Credentials,
	})
	analysisClient = analysis.NewClient(
		nil,
		sqs.NewFromConfig(cfg),
	)
}

func handler(
	ctx context.Context,
	event events.APIGatewayProxyRequest,
) (
	events.APIGatewayProxyResponse,
	error,
) {
	connectionId := event.PathParameters["id"]
	var req dtos.FenAnalysisSubmission
	err := json.Unmarshal([]byte(event.Body), &req)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to unmarshal body: %w", err)
	}

	analysisJson, err := json.Marshal(req.Results)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to marshal analysis: %w", err)
	}
	_, err = apigatewayClient.PostToConnection(
		ctx,
		&apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: aws.String(connectionId),
			Data:         analysisJson,
		},
	)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to post to connect: %w", err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
	}, nil
}

func main() {
	lambda.Start(handler)
}
