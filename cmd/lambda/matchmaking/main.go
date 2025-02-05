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
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/bucket-sort/slchess/internal/domains/entities"
	"github.com/bucket-sort/slchess/pkg/utils"
	"github.com/redis/go-redis/v9"
)

var (
	apiGatewayClient  *apigatewaymanagementapi.Client
	elasticacheClient *elasticache.Client
	dynamoClient      *dynamodb.Client
	ctx               context.Context

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
	elasticacheClient = elasticache.NewFromConfig(cfg)
}

// Handle matchmaking requests
func handler(event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionId := event.RequestContext.ConnectionID
	// TODO: retrieve lower and upper bound range
	ratingLowerBound := -150
	ratingUpperBound := 150

	// Get user ID from DynamoDB
	response, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("Connections"),
		Key: map[string]types.AttributeValue{
			"Id": &types.AttributeValueMemberS{Value: connectionId},
		},
	})
	if err != nil || response.Item == nil {
		return events.APIGatewayProxyResponse{StatusCode: 401, Body: "Unauthorized"}, nil
	}

	userId := response.Item["UserId"].(*types.AttributeValueMemberS).Value

	var body map[string]interface{}
	json.Unmarshal([]byte(event.Body), &body)
	userRating := body["rating"].(int)

	redisAddr, err := getRedisEndpoint(ctx)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to queue player"}, nil
	}

	// Create a Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Test connection
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	fmt.Println("Connected to Redis:", pong)

	// Attempt matchmaking
	minRating := userRating - ratingLowerBound
	maxRating := userRating + ratingUpperBound
	opponentId, err := findMatch(rdb, "matchmaking_set", minRating, maxRating, userId, userRating)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Matchmaking error"}, nil
	}

	// If no match found, queue the player by caching the matchmaking ticket
	if opponentId == "" {
		_, err = apiGatewayClient.PostToConnection(context.TODO(), &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: &connectionId,
			Data:         []byte("{result: 'queued'}"),
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
	// TODO: retrieve game server ip
	serverIp := "192.168.0.1"

	createdAt := time.Now()
	sessionId := utils.GenerateUUID()
	_, err := dynamoClient.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String("ActiveMatches"),
		Item: map[string]types.AttributeValue{
			"MatchId":   &types.AttributeValueMemberS{Value: sessionId},
			"Player1Id": &types.AttributeValueMemberS{Value: player1Id},
			"Player2Id": &types.AttributeValueMemberS{Value: player2Id},
			"Server":    &types.AttributeValueMemberS{Value: serverIp},
			"CreatedAt": &types.AttributeValueMemberS{Value: createdAt.String()},
		},
	})
	if err != nil {
		return entities.Session{}, err
	}
	return entities.Session{
		Id:        sessionId,
		Player1Id: player1Id,
		Player2Id: player2Id,
		Server:    serverIp,
		CreatedAt: createdAt,
	}, nil
}

// Get Redis endpoint dynamically
func getRedisEndpoint(ctx context.Context) (string, error) {
	output, err := elasticacheClient.DescribeServerlessCaches(ctx, &elasticache.DescribeServerlessCachesInput{
		ServerlessCacheName: aws.String("SlchessCache"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe cluster: %v", err)
	}

	if len(output.ServerlessCaches) > 0 {
		endpoint := output.ServerlessCaches[0].Endpoint
		return fmt.Sprintf("%s:%d", *endpoint.Address, endpoint.Port), nil
	}

	return "", fmt.Errorf("no cache nodes found")
}

func main() {
	lambda.Start(handler)
}
