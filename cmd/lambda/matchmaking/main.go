package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/bucket-sort/slchess/internal/domains/entities"
	"github.com/bucket-sort/slchess/pkg/utils"
	"github.com/redis/go-redis/v9"
)

var (
	apiGatewayClient *apigatewaymanagementapi.Client
	dynamoClient     *dynamodb.Client
	ctx              context.Context

	ErrNoMatchFound = errors.New("failed to matchmaking")
)

func init() {
	ctx = context.Background()
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
	apiEndpoint := fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/Prod", os.Getenv("AWS_API_ID"), os.Getenv("AWS_REGION"))
	apiGatewayClient = apigatewaymanagementapi.New(apigatewaymanagementapi.Options{
		BaseEndpoint: aws.String(apiEndpoint),
		Region:       os.Getenv("AWS_REGION"),
		Credentials:  cfg.Credentials,
	})
}

// Handle matchmaking requests
func handler(event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionId := event.RequestContext.ConnectionID

	// Get user ID from DynamoDB
	response, err := dynamoClient.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String("Connections"),
		Key: map[string]types.AttributeValue{
			"connectionId": &types.AttributeValueMemberS{Value: connectionId},
		},
	})
	if err != nil || response.Item == nil {
		return events.APIGatewayProxyResponse{StatusCode: 401, Body: "Unauthorized"}, nil
	}

	userId := response.Item["userId"].(*types.AttributeValueMemberS).Value

	var body map[string]interface{}
	json.Unmarshal([]byte(event.Body), &body)
	userRating := body["rating"].(int)

	redisAddr := "your-redis-endpoint.cache.amazonaws.com:6379"

	// Create a Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // Set password if using authentication
		DB:       0,  // Default DB
	})

	// Test connection
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	fmt.Println("Connected to Redis:", pong)

	// TODO: perform matchmaking logic, query AWS Elasticache
	minRating := userRating - 150
	maxRating := userRating + 150
	opponentId, err := findMatch(rdb, "matchmaking_set", minRating, maxRating, userId, userRating)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Matchmaking error"}, nil
	}

	if opponentId == "" {
		_, err = apiGatewayClient.PostToConnection(context.TODO(), &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: &connectionId,
			Data:         []byte("{result: 'failed'}"),
		})
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to send message"}, nil
		}
	} else {
		// Create new session
		session, err := createSession(userId, opponentId)
		if err != nil {
		}
		data, _ := json.Marshal(session)
		_, err = apiGatewayClient.PostToConnection(context.TODO(), &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: &connectionId,
			Data:         data,
		})
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to send message"}, nil
		}
	}

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

// Matchmaking function using go-redis commands
func findMatch(client *redis.Client, key string, minRating, maxRating int, playerId string, playerRating int) (string, error) {
	// Find an opponent within the rating range
	matches, err := client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min:    strconv.Itoa(minRating),
		Max:    strconv.Itoa(maxRating),
		Offset: 0,
		Count:  1, // Only get the first match
	}).Result()
	if err != nil {
		return "", err
	}

	if len(matches) > 0 {
		// Match found, remove it from the queue
		playerId := matches[0]
		_, err := client.ZRem(ctx, key, playerId).Result()
		if err != nil {
			return "", err
		}
		return playerId, nil
	}
	// No match found, add the current player to the queue
	_, err = client.ZAdd(ctx, "matchmaking_set", redis.Z{
		Score:  float64(playerRating),
		Member: playerId,
	}).Result()
	if err != nil {
		return "", err
	}

	return "", nil
}

func createSession(player1Id, player2Id string) (entities.Session, error) {
	sessionId := utils.GenerateUUID()
	_, err := dynamoClient.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String("ActiveSessions"),
		Item: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: sessionId},
		},
	})
	if err != nil {
	}
	return entities.Session{
		Id:        sessionId,
		Player1Id: player1Id,
		Player2Id: player2Id,
		Server:    "",
		CreatedAt: time.Now(),
	}, nil
}

func respond(ctx context.Context, connectionId string, data []byte) {
}

func main() {
	lambda.Start(handler)
}
