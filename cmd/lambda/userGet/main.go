package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/chess-vn/slchess/internal/domains/dtos"
	"github.com/chess-vn/slchess/internal/domains/entities"
	"github.com/chess-vn/slchess/pkg/logging"
	"go.uber.org/zap"
)

var dynamoClient *dynamodb.Client

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	userId := mustAuth(event.RequestContext.Authorizer)
	targetId := event.PathParameters["id"]
	if targetId == "" {
		targetId = userId
	}
	targetProfile, err := getUserProfile(ctx, targetId)
	if err != nil {
		logging.Error("Failed to get user profile", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}
	targetRating, err := getUserRating(ctx, targetId)
	if err != nil {
		logging.Error("Failed to get user rating", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	// If users request their own information, return in full
	var getFull bool
	if userId == targetId {
		getFull = true
	}
	user := dtos.UserResponseFromEntities(targetProfile, targetRating, getFull)
	userJson, err := json.Marshal(user)
	if err != nil {
		logging.Error("Failed to get user", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}
	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(userJson)}, nil
}

func getUserProfile(ctx context.Context, userId string) (entities.UserProfile, error) {
	userProfileOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("UserProfiles"),
		Key: map[string]types.AttributeValue{
			"UserId": &types.AttributeValueMemberS{
				Value: userId,
			},
		},
	})
	if err != nil {
		return entities.UserProfile{}, err
	}
	var userProfile entities.UserProfile
	if err := attributevalue.UnmarshalMap(userProfileOutput.Item, &userProfile); err != nil {
		return entities.UserProfile{}, err
	}
	return userProfile, nil
}

func getUserRating(ctx context.Context, userId string) (entities.UserRating, error) {
	userRatingOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("UserRatings"),
		Key: map[string]types.AttributeValue{
			"UserId": &types.AttributeValueMemberS{
				Value: userId,
			},
		},
	})
	if err != nil {
		return entities.UserRating{}, err
	}
	var userRating entities.UserRating
	if err := attributevalue.UnmarshalMap(userRatingOutput.Item, &userRating); err != nil {
		return entities.UserRating{}, err
	}
	return userRating, nil
}

func mustAuth(authorizer map[string]interface{}) string {
	v, exists := authorizer["claims"]
	if !exists {
		panic("no authorizer claims")
	}
	claims, ok := v.(map[string]interface{})
	if !ok {
		panic("claims must be of type map")
	}
	userId, ok := claims["sub"].(string)
	if !ok {
		panic("invalid sub")
	}
	return userId
}

func main() {
	lambda.Start(handler)
}
