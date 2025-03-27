package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/chess-vn/slchess/internal/domains/entities"
)

var ErrFriendshipNotFound = fmt.Errorf("friendship not found")

type FriendshipUpdateOptions struct {
	Status         *string
	ConversationId *string
	StartedAt      *time.Time
}

func (client *Client) GetFriendship(ctx context.Context, userId string, friendId string) (entities.Friendship, error) {
	output, err := client.dynamodb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: client.cfg.FriendshipsTableName,
		Key: map[string]types.AttributeValue{
			"UserId": &types.AttributeValueMemberS{
				Value: userId,
			},
			"FriendId": &types.AttributeValueMemberS{
				Value: friendId,
			},
		},
	})
	if err != nil {
		return entities.Friendship{}, err
	}
	if output.Item == nil {
		return entities.Friendship{}, ErrFriendshipNotFound
	}
	var friendship entities.Friendship
	if err := attributevalue.UnmarshalMap(output.Item, &friendship); err != nil {
		return entities.Friendship{}, err
	}
	return friendship, nil
}

func (client *Client) FetchFriendships(
	ctx context.Context,
	userId string,
	lastKey map[string]types.AttributeValue,
	limit int32,
) (
	[]entities.Friendship,
	map[string]types.AttributeValue,
	error,
) {
	input := &dynamodb.QueryInput{
		TableName:              client.cfg.FriendshipsTableName,
		KeyConditionExpression: aws.String("UserId = :userId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userId": &types.AttributeValueMemberS{Value: userId},
		},
		ExclusiveStartKey: lastKey,
		ScanIndexForward:  aws.Bool(true),
		Limit:             aws.Int32(limit),
	}
	output, err := client.dynamodb.Query(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var friendships []entities.Friendship
	err = attributevalue.UnmarshalListOfMaps(output.Items, &friendships)
	if err != nil {
		return nil, nil, err
	}

	return friendships, output.LastEvaluatedKey, nil
}

func (client *Client) PutFriendship(ctx context.Context, friendship entities.Friendship) error {
	av, err := attributevalue.MarshalMap(friendship)
	if err != nil {
		return fmt.Errorf("failed to marshal evaluation map: %w", err)
	}

	_, err = client.dynamodb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: client.cfg.FriendshipsTableName,
		Item:      av,
	})
	if err != nil {
		return fmt.Errorf("failed to put friendship: %w", err)
	}
	return nil
}

func (client *Client) UpdateFriendship(
	ctx context.Context,
	userId,
	friendId string,
	opts FriendshipUpdateOptions,
) error {
	updateExpression := []string{}
	expressionAttributeValues := map[string]types.AttributeValue{}

	if opts.Status != nil {
		updateExpression = append(updateExpression, "Status = :status")
		expressionAttributeValues[":status"] = &types.AttributeValueMemberS{
			Value: *opts.Status,
		}
	}
	if opts.ConversationId != nil {
		updateExpression = append(updateExpression, "ConversationId = :conversationId")
		expressionAttributeValues[":conversationId"] = &types.AttributeValueMemberS{
			Value: *opts.ConversationId,
		}
	}
	if opts.StartedAt != nil {
		updateExpression = append(updateExpression, "StartedAt = :startedAt")
		expressionAttributeValues[":startedAt"] = &types.AttributeValueMemberS{
			Value: opts.StartedAt.Format(time.RFC3339),
		}
	}

	_, err := client.dynamodb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: client.cfg.FriendshipsTableName,
		Key: map[string]types.AttributeValue{
			"UserId": &types.AttributeValueMemberS{
				Value: userId,
			},
			"FriendId": &types.AttributeValueMemberS{
				Value: friendId,
			},
		},
		UpdateExpression:          aws.String("SET " + strings.Join(updateExpression, ", ")),
		ExpressionAttributeValues: expressionAttributeValues,
	})
	if err != nil {
		return err
	}

	return nil
}

func (client *Client) DeleteFriendship(ctx context.Context, userId, friendId string) error {
	_, err := client.dynamodb.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: client.cfg.FriendshipsTableName,
		Key: map[string]types.AttributeValue{
			"UserId":   &types.AttributeValueMemberS{Value: userId},
			"FriendId": &types.AttributeValueMemberS{Value: friendId},
		},
	})
	if err != nil {
		return err
	}
	return nil
}
