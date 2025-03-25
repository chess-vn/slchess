package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/chess-vn/slchess/internal/app/lichess"
	"github.com/chess-vn/slchess/internal/aws/analysis"
	"github.com/chess-vn/slchess/internal/aws/storage"
	"github.com/chess-vn/slchess/internal/domains/dtos"
)

var (
	storageClient    *storage.Client
	analysisClient   *analysis.Client
	lichessClient    *lichess.Client
	apigatewayClient *apigatewaymanagementapi.Client

	region            = os.Getenv("AWS_REGION")
	websocketApiId    = os.Getenv("WEBSOCKET_API_ID")
	websocketApiStage = os.Getenv("WEBSOCKET_API_STAGE")
)

type Body struct {
	Action  string `json:"action"`
	Message string `json:"message"`
}

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	storageClient = storage.NewClient(dynamodb.NewFromConfig(cfg))
	analysisClient = analysis.NewClient(
		nil,
		sqs.NewFromConfig(cfg),
	)
	lichessClient = lichess.NewClient()
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
}

func handler(
	ctx context.Context,
	event events.APIGatewayWebsocketProxyRequest,
) (
	events.APIGatewayProxyResponse,
	error,
) {
	connectionId := aws.String(event.RequestContext.ConnectionID)
	var body Body
	err := json.Unmarshal([]byte(event.Body), &body)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to unmarshal body: %w", err)
	}
	fen := body.Message

	// Query from lichess
	eval, err := lichessClient.CloudEvaluate(fen)
	if err == nil {
		resp := dtos.EvaluationResponseFromEntity(eval)
		respJson, err := json.Marshal(resp)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
			}, fmt.Errorf("failed to marshal response: %w", err)
		}
		_, err = apigatewayClient.PostToConnection(
			ctx,
			&apigatewaymanagementapi.PostToConnectionInput{
				ConnectionId: connectionId,
				Data:         respJson,
			},
		)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
			}, fmt.Errorf("failed to post to connection: %w", err)
		}
	} else {
		if !errors.Is(err, lichess.ErrEvaluationNotFound) {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
			}, fmt.Errorf("failed to query lichess api : %w", err)
		}
	}

	// If not found, check in dynamodb table
	eval, err = storageClient.GetEvaluation(ctx, fen)
	if err == nil {
		resp := dtos.EvaluationResponseFromEntity(eval)
		respJson, err := json.Marshal(resp)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
			}, fmt.Errorf("failed to marshal response: %w", err)
		}
		_, err = apigatewayClient.PostToConnection(
			ctx,
			&apigatewaymanagementapi.PostToConnectionInput{
				ConnectionId: connectionId,
				Data:         respJson,
			},
		)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
			}, fmt.Errorf("failed to post to connection: %w", err)
		}
	} else {
		if !errors.Is(err, storage.ErrEvaluationNotFound) {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
			}, fmt.Errorf("failed to get evaluation: %w", err)
		}
	}

	// If no cached evaluation found, submit request for new evaluation
	err = analysisClient.SubmitEvaluationRequest(ctx, dtos.EvaluationRequest{
		ConnectionId: *connectionId,
		Fen:          fen,
	})
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to submit evaluation request: %w", err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
	}, nil
}

func main() {
	lambda.Start(handler)
}
