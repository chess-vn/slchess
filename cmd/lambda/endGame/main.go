package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/chess-vn/slchess/internal/domains/dtos"
	"github.com/chess-vn/slchess/internal/domains/entities"
)

var dynamoClient *dynamodb.Client

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, event json.RawMessage) error {
	// Get match data
	var matchRecordReq dtos.MatchRecordRequest
	if err := json.Unmarshal(event, &matchRecordReq); err != nil {
		return fmt.Errorf("failed to unmarshal request: %w", err)
	}

	matchRecord := dtos.MatchRecordRequestToEntity(matchRecordReq)

	_, err := dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("ActiveMatches"),
		Key: map[string]types.AttributeValue{
			"MatchId": &types.AttributeValueMemberS{Value: matchRecord.MatchId},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete active match: %w", err)
	}

	_, err = dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("UserMatches"),
		Key: map[string]types.AttributeValue{
			"UserId": &types.AttributeValueMemberS{Value: matchRecord.Players[0].Id},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete user match: %w", err)
	}

	_, err = dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("UserMatches"),
		Key: map[string]types.AttributeValue{
			"UserId": &types.AttributeValueMemberS{Value: matchRecord.Players[1].Id},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete user match: %w", err)
	}

	_, err = dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("SpectatorConversations"),
		Key: map[string]types.AttributeValue{
			"MatchId": &types.AttributeValueMemberS{Value: matchRecord.MatchId},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete spectator conversation: %w", err)
	}

	av, err := attributevalue.MarshalMap(matchRecord)
	if err != nil {
		return fmt.Errorf("failed to marshal match record map: %w", err)
	}
	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("MatchRecords"),
		Item:      av,
	})
	if err != nil {
		return fmt.Errorf("failed to put match record: %w", err)
	}

	player1Rating := entities.UserRating{
		UserId:       matchRecordReq.Players[0].Id,
		PartitionKey: "UserRatings",
		Rating:       matchRecordReq.Players[0].NewRating,
		RD:           matchRecordReq.Players[0].NewRD,
	}
	player1RatingAv, err := attributevalue.MarshalMap(player1Rating)
	if err != nil {
		return fmt.Errorf("failed to marshal user rating map: %w", err)
	}
	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("UserRatings"),
		Item:      player1RatingAv,
	})
	if err != nil {
		return fmt.Errorf("failed to put user rating: %w", err)
	}

	player2Rating := entities.UserRating{
		UserId:       matchRecordReq.Players[1].Id,
		PartitionKey: "UserRatings",
		Rating:       matchRecordReq.Players[1].NewRating,
		RD:           matchRecordReq.Players[1].NewRD,
	}
	player2RatingAv, err := attributevalue.MarshalMap(player2Rating)
	if err != nil {
		return fmt.Errorf("failed to marshal user rating map: %w", err)
	}
	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("UserRatings"),
		Item:      player2RatingAv,
	})
	if err != nil {
		return fmt.Errorf("failed to put user rating: %w", err)
	}

	player1MatchResult := entities.MatchResult{
		UserId:         matchRecordReq.Players[0].Id,
		MatchId:        matchRecordReq.MatchId,
		OpponentId:     matchRecordReq.Players[1].Id,
		OpponentRating: matchRecordReq.Players[1].OldRating,
		OpponentRD:     matchRecordReq.Players[1].OldRD,
		Result:         matchRecordReq.Results[0],
		Timestamp:      matchRecordReq.EndedAt.Format(time.RFC3339),
	}
	attributevalue.MarshalMap(&player1MatchResult)
	player1MatchResultAv, err := attributevalue.MarshalMap(player1MatchResult)
	if err != nil {
		return fmt.Errorf("failed to marshal match result map: %w", err)
	}
	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("MatchResults"),
		Item:      player1MatchResultAv,
	})
	if err != nil {
		return fmt.Errorf("failed to put match result: %w", err)
	}

	player2MatchResult := entities.MatchResult{
		UserId:         matchRecordReq.Players[1].Id,
		MatchId:        matchRecordReq.MatchId,
		OpponentId:     matchRecordReq.Players[0].Id,
		OpponentRating: matchRecordReq.Players[0].OldRating,
		OpponentRD:     matchRecordReq.Players[0].OldRD,
		Result:         matchRecordReq.Results[1],
		Timestamp:      matchRecordReq.EndedAt.Format(time.RFC3339),
	}
	attributevalue.MarshalMap(&player2MatchResult)
	player2MatchResultAv, err := attributevalue.MarshalMap(player2MatchResult)
	if err != nil {
		return fmt.Errorf("failed to marshal match result map: %w", err)
	}
	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("MatchResults"),
		Item:      player2MatchResultAv,
	})
	if err != nil {
		return fmt.Errorf("failed to put match result: %w", err)
	}
	return nil
}

func main() {
	lambda.Start(handler)
}
