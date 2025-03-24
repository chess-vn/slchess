package analysis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/chess-vn/slchess/internal/domains/dtos"
	"github.com/chess-vn/slchess/internal/domains/entities"
)

func (client *Client) SubmitFenAnalyseRequest(
	ctx context.Context,
	request dtos.FenAnalyseRequest,
) error {
	reqJson, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	_, err = client.sqs.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    client.cfg.FenQueueUrl,
		MessageBody: aws.String(string(reqJson)),
	})
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	return nil
}

func (client *Client) AcquireFenAnalysisWork(ctx context.Context) (entities.FenAnalysisWork, error) {
	output, err := client.sqs.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            client.cfg.FenQueueUrl,
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     20,
		VisibilityTimeout:   60,
	})
	if err != nil {
		return entities.FenAnalysisWork{},
			fmt.Errorf("failed to receive message: %w", err)
	}
	if len(output.Messages) < 1 {
		return entities.FenAnalysisWork{},
			fmt.Errorf("invalid message array length")
	}

	var req dtos.FenAnalyseRequest
	err = json.Unmarshal([]byte(*output.Messages[0].Body), &req)
	if err != nil {
		return entities.FenAnalysisWork{},
			fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return dtos.FenAnalysisRequestToEntity(req), nil
}
