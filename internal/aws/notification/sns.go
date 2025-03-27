package notification

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

func (client *Client) SendPushNotification(
	ctx context.Context,
	endpointArn,
	message string,
) error {
	_, err := client.sns.Publish(ctx, &sns.PublishInput{
		Message:          aws.String(message),
		MessageStructure: aws.String("json"),
		TargetArn:        aws.String(endpointArn),
	})
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}
	return nil
}
