package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/chess-vn/slchess/internal/aws/analysis"
	"github.com/chess-vn/slchess/internal/domains/dtos"
)

var analysisClient *analysis.Client

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	analysisClient = analysis.NewClient(
		nil,
		sqs.NewFromConfig(cfg),
	)
}

func handler(
	ctx context.Context,
	event events.APIGatewayWebsocketProxyRequest,
) (
	events.APIGatewayProxyResponse,
	error,
) {
	err := analysisClient.SubmitFenAnalyseRequest(ctx, dtos.FenAnalyseRequest{
		Id:  event.RequestContext.ConnectionID,
		Fen: event.Body,
	})
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to submit request : %w", err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
	}, nil
}

func main() {
	lambda.Start(handler)
}
