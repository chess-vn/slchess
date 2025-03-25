package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/chess-vn/slchess/internal/aws/analysis"
	"github.com/chess-vn/slchess/internal/aws/storage"
	"github.com/chess-vn/slchess/internal/domains/dtos"
)

var (
	apigatewayClient *apigatewaymanagementapi.Client
	analysisClient   *analysis.Client
	storageClient    *storage.Client

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
	storageClient = storage.NewClient(dynamodb.NewFromConfig(cfg))
}

func handler(
	ctx context.Context,
	event events.APIGatewayProxyRequest,
) (
	events.APIGatewayProxyResponse,
	error,
) {
	var submission dtos.EvaluationSubmission
	err := json.Unmarshal([]byte(event.Body), &submission)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to unmarshal body: %w", err)
	}

	eval := dtos.EvaluationSubmissionToEntity(submission)

	evalJson, err := json.Marshal(dtos.EvaluationResponseFromEntity(eval))
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to marshal analysis: %w", err)
	}
	_, err = apigatewayClient.PostToConnection(
		ctx,
		&apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: aws.String(submission.ConnectionId),
			Data:         evalJson,
		},
	)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to post to connect: %w", err)
	}

	err = storageClient.PutEvaluation(ctx, eval, 24*time.Hour)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to put evaluation: %w", err)
	}

	analysisClient.RemoveEvaluationWork(ctx, submission.ReceiptHandle)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
	}, nil
}

func main() {
	lambda.Start(handler)
}
