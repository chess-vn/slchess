package storage

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/chess-vn/slchess/internal/domains/entities"
)

var ErrApplicationEndpointNotFound = fmt.Errorf("application endpoint not found")

func (client *Client) GetApplicationEndpoint(
	ctx context.Context,
	userId string,
) (
	entities.ApplicationEndpoint,
	error,
) {
	output, err := client.dynamodb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: client.cfg.ApplicationEndpointsTableName,
		Key: map[string]types.AttributeValue{
			"UserId": &types.AttributeValueMemberS{
				Value: userId,
			},
		},
	})
	if err != nil {
		return entities.ApplicationEndpoint{}, err
	}
	if output.Item == nil {
		return entities.ApplicationEndpoint{}, ErrApplicationEndpointNotFound
	}
	var endpoint entities.ApplicationEndpoint
	if err := attributevalue.UnmarshalMap(output.Item, &endpoint); err != nil {
		return entities.ApplicationEndpoint{}, err
	}
	return endpoint, nil
}

func (client *Client) PutApplicationEndpoint(ctx context.Context, endpoint entities.ApplicationEndpoint) error {
	av, err := attributevalue.MarshalMap(endpoint)
	if err != nil {
		return fmt.Errorf("failed to marshal map: %w", err)
	}
	_, err = client.dynamodb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: client.cfg.ApplicationEndpointsTableName,
		Item:      av,
	})
	if err != nil {
		return err
	}
	return nil
}
