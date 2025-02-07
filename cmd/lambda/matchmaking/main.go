package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
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
)

var (
	apiGatewayClient  *apigatewaymanagementapi.Client
	elasticacheClient *elasticache.Client
	dynamoClient      *dynamodb.Client
	ctx               context.Context

	ErrNoMatchFound = errors.New("failed to matchmaking")
)

type jwt struct {
	Claims claims `json:"claims"`
}

type claims struct {
	Name string `json:"name"`
}

func init() {
	ctx = context.Background()
	cfg, _ := config.LoadDefaultConfig(ctx)
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
func handler(event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	v, exists := event.RequestContext.Authorizer["jwt"]
	if !exists {
		return events.APIGatewayProxyResponse{
			StatusCode: 401,
			Body:       "Unauthorized: No valid JWT token",
		}, nil
	}
	log.Println(v)
	var jwtData jwt
	json.Unmarshal(v.([]byte), &jwtData)
	userId := jwtData.Claims.Name

	var body map[string]interface{}
	json.Unmarshal([]byte(event.Body), &body)
	userRating := body["rating"].(float64)
	ratingLowerBound := body["lower_bound"].(float64)
	ratingUpperBound := body["upper_bound"].(float64)

	// Attempt matchmaking
	minRating := int(userRating + ratingLowerBound)
	maxRating := int(userRating + ratingUpperBound)
	log.Printf("Attempt matchmaking in rating range: %d - %d\n", minRating, maxRating)
	opponentIds, err := findOpponents(minRating, maxRating, userId, int(userRating))
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Matchmaking error"}, nil
	}

	// If no match found, queue the player by caching the matchmaking ticket
	if len(opponentIds) == 0 {
		log.Println("No match found. Start queuing")
		return events.APIGatewayProxyResponse{StatusCode: http.StatusAccepted, Body: "Queued"}, nil
	}

	// Try to create new match
	for _, opponentId := range opponentIds {
		match, err := createMatch(ctx, userId, opponentId)
		if err != nil {
			continue
		}
		log.Printf("Match found: %s: %s - %s", match.Id, match.Player1Id, match.Player2Id)
		data, _ := json.Marshal(match)

		return events.APIGatewayProxyResponse{StatusCode: http.StatusCreated, Body: string(data)}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
}

// Matchmaking function using go-redis commands
func findOpponents(minRating, maxRating int, userId string, userRating int) ([]string, error) {
	output, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
		TableName:        aws.String("MatchmakingRequests"),
		FilterExpression: aws.String("Rating BETWEEN :minRating AND :maxRating"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userId":    &types.AttributeValueMemberS{Value: userId},
			":minRating": &types.AttributeValueMemberN{Value: strconv.Itoa(minRating)},
			":maxRating": &types.AttributeValueMemberN{Value: strconv.Itoa(maxRating)},
		},
	})
	if err != nil {
		return nil, err
	}

	var matches []string
	if output.Count > 0 {
		for _, item := range output.Items {
			if v, ok := item["UserId"].(*types.AttributeValueMemberS); ok && v.Value != userId {
				matches = append(matches, v.Value)
			}
		}
	} else {
		// No match found, add the user ticket to the queue
		_, err := dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String("MatchmakingRequests"),
			Item: map[string]types.AttributeValue{
				"UserId":    &types.AttributeValueMemberS{Value: userId},
				"Rating":    &types.AttributeValueMemberN{Value: strconv.Itoa(userRating)},
				"MinRating": &types.AttributeValueMemberN{Value: strconv.Itoa(minRating)},
				"MaxRating": &types.AttributeValueMemberN{Value: strconv.Itoa(maxRating)},
			},
		})
		if err != nil {
			return nil, err
		}
	}

	return matches, nil
}

func createMatch(ctx context.Context, userId, opponentId string) (entities.Match, error) {
	// TODO: retrieve game server ip
	serverIp := "192.168.0.1"

	createdAt := time.Now()
	matchId := utils.GenerateUUID()
	_, err := dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("ActiveMatches"),
		Item: map[string]types.AttributeValue{
			"MatchId":   &types.AttributeValueMemberS{Value: matchId},
			"Player1Id": &types.AttributeValueMemberS{Value: userId},
			"Player2Id": &types.AttributeValueMemberS{Value: opponentId},
			"Server":    &types.AttributeValueMemberS{Value: serverIp},
			"CreatedAt": &types.AttributeValueMemberS{Value: createdAt.String()},
		},
	})
	if err != nil {
		return entities.Match{}, err
	}
	// Match created, remove opponent ticket from the queue
	dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("MatchmakingRequests"),
		Key: map[string]types.AttributeValue{
			"UserId": &types.AttributeValueMemberS{Value: opponentId},
		},
	})
	return entities.Match{
		Id:        matchId,
		Player1Id: userId,
		Player2Id: opponentId,
		Server:    serverIp,
		CreatedAt: createdAt,
	}, nil
}

// Get Redis endpoint dynamically
func getRedisEndpoint(ctx context.Context) (string, error) {
	output, err := elasticacheClient.DescribeServerlessCaches(ctx, &elasticache.DescribeServerlessCachesInput{
		ServerlessCacheName: aws.String(os.Getenv("CACHE_NAME")),
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe cache: %v", err)
	}

	if len(output.ServerlessCaches) > 0 {
		endpoint := output.ServerlessCaches[0].Endpoint
		return fmt.Sprintf("%s:%d", *endpoint.Address, *endpoint.Port), nil
	}

	return "", fmt.Errorf("no cache nodes found")
}

func main() {
	lambda.Start(handler)
}
