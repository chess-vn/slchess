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
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snsTypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/bucket-sort/slchess/internal/domains/entities"
	"github.com/bucket-sort/slchess/pkg/utils"
)

var (
	apiGatewayClient *apigatewaymanagementapi.Client
	dynamoClient     *dynamodb.Client
	snsClient        *sns.Client
	ctx              context.Context
	timeLayout       string

	ErrNoMatchFound    = errors.New("failed to matchmaking")
	ErrInvalidGameMode = errors.New("invalid game mode")
)

func init() {
	ctx = context.Background()
	timeLayout = "2006-01-02 15:04:05.999999999 -0700 MST"
	cfg, _ := config.LoadDefaultConfig(ctx)
	dynamoClient = dynamodb.NewFromConfig(cfg)
	apiEndpoint := fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/Prod", os.Getenv("AWS_API_ID"), os.Getenv("AWS_REGION"))
	apiGatewayClient = apigatewaymanagementapi.New(apigatewaymanagementapi.Options{
		BaseEndpoint: aws.String(apiEndpoint),
		Region:       os.Getenv("AWS_REGION"),
		Credentials:  cfg.Credentials,
	})
	snsClient = sns.NewFromConfig(cfg)
}

// Handle matchmaking requests
func handler(event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	v, exists := event.RequestContext.Authorizer["claims"]
	if !exists {
		return events.APIGatewayProxyResponse{
			StatusCode: 401,
			// Body:       "Unauthorized: No valid JWT token",
			Body: fmt.Sprintf("%v", event.RequestContext),
		}, nil
	}
	claims := v.(map[string]interface{})
	userId := claims["cognito:username"].(string)

	var body map[string]interface{}
	json.Unmarshal([]byte(event.Body), &body)
	userRating := body["rating"].(float64)
	ratingLowerBound := body["lower_bound"].(float64)
	ratingUpperBound := body["upper_bound"].(float64)
	gameMode := body["mode"].(string)

	// Check if user already in a match
	match, exist, err := checkForActiveMatch(ctx, userId, gameMode)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Matchmaking error: %v", err)}, nil
	}
	if exist {
		data, _ := json.Marshal(match)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusConflict, Body: string(data)}, nil
	}

	// Attempt matchmaking
	minRating := int(userRating + ratingLowerBound)
	maxRating := int(userRating + ratingUpperBound)
	log.Printf("Attempt matchmaking in rating range: %d - %d\n", minRating, maxRating)
	opponentIds, err := findOpponents(minRating, maxRating, userId, int(userRating), gameMode)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Matchmaking error: %v", err)}, nil
	}

	// If no match found, queue the player by caching the matchmaking ticket
	if len(opponentIds) == 0 {
		log.Println("No match found. Start queuing")
		return events.APIGatewayProxyResponse{StatusCode: http.StatusAccepted, Body: "Queued"}, nil
	}

	var errs []error
	// Try to create new match
	for _, opponentId := range opponentIds {
		match, err := createMatch(ctx, userId, opponentId, gameMode)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", opponentId, err))
			continue
		}
		log.Printf("Match found: %s: %s - %s\n", match.Id, match.Player1Id, match.Player2Id)
		data, _ := json.Marshal(match)

		// Notify the opponent about the match
		err = notifyUser(userId, data)
		if err != nil {
			log.Printf("Failed to notify user %s:%v\n", opponentId, err)
		}

		return events.APIGatewayProxyResponse{StatusCode: http.StatusCreated, Body: string(data)}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: fmt.Sprintf("%v", errs)}, nil
}

// Matchmaking function using go-redis commands
func findOpponents(minRating, maxRating int, userId string, userRating int, gameMode string) ([]string, error) {
	output, err := dynamoClient.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String("MatchmakingTickets"),
		FilterExpression: aws.String("MinRating >= :minRating AND MaxRating <= :maxRating AND Mode = :mode"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":minRating": &types.AttributeValueMemberN{Value: strconv.Itoa(minRating)},
			":maxRating": &types.AttributeValueMemberN{Value: strconv.Itoa(maxRating)},
			":mode":      &types.AttributeValueMemberS{Value: gameMode},
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
			TableName: aws.String("MatchmakingTickets"),
			Item: map[string]types.AttributeValue{
				"UserId":    &types.AttributeValueMemberS{Value: userId},
				"Rating":    &types.AttributeValueMemberN{Value: strconv.Itoa(userRating)},
				"MinRating": &types.AttributeValueMemberN{Value: strconv.Itoa(minRating)},
				"MaxRating": &types.AttributeValueMemberN{Value: strconv.Itoa(maxRating)},
				"Mode":      &types.AttributeValueMemberS{Value: gameMode},
			},
		})
		if err != nil {
			return nil, err
		}
	}

	return matches, nil
}

func createMatch(ctx context.Context, userId, opponentId, gameMode string) (entities.Match, error) {
	// TODO: retrieve game server ip
	serverIp := "192.168.0.1"

	match := entities.Match{
		Id:        utils.GenerateUUID(),
		Player1Id: userId,
		Player2Id: opponentId,
		Mode:      gameMode,
		Server:    serverIp,
		CreatedAt: time.Now(),
	}

	_, err := dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("UserMatches"),
		Item: map[string]types.AttributeValue{
			"UserId":  &types.AttributeValueMemberS{Value: userId},
			"MatchId": &types.AttributeValueMemberS{Value: match.Id},
			"Mode":    &types.AttributeValueMemberS{Value: match.Mode},
		},
	})
	if err != nil {
		return entities.Match{}, err
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("ActiveMatches"),
		Item: map[string]types.AttributeValue{
			"MatchId":   &types.AttributeValueMemberS{Value: match.Id},
			"Player1Id": &types.AttributeValueMemberS{Value: match.Player1Id},
			"Player2Id": &types.AttributeValueMemberS{Value: match.Player2Id},
			"Mode":      &types.AttributeValueMemberS{Value: match.Mode},
			"Server":    &types.AttributeValueMemberS{Value: match.Server},
			"CreatedAt": &types.AttributeValueMemberS{Value: match.CreatedAt.Format(timeLayout)},
		},
	})
	if err != nil {
		return entities.Match{}, err
	}
	// Match created, remove opponent ticket from the queue
	dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("MatchmakingTickets"),
		Key: map[string]types.AttributeValue{
			"UserId": &types.AttributeValueMemberS{Value: opponentId},
		},
	})
	return match, nil
}

func checkForActiveMatch(ctx context.Context, userId string, gameMode string) (entities.Match, bool, error) {
	output, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("UserMatches"),
		Key: map[string]types.AttributeValue{
			"UserId": &types.AttributeValueMemberS{
				Value: userId,
			},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return entities.Match{}, false, err
	}

	matchId := output.Item["MatchId"].(*types.AttributeValueMemberS).Value

	output, err = dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("ActiveMatches"),
		Key: map[string]types.AttributeValue{
			"MatchId": &types.AttributeValueMemberS{Value: matchId},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return entities.Match{}, false, err
	}
	if output.Item == nil {
		return entities.Match{}, false, nil
	}

	createdAt, err := time.Parse(timeLayout, output.Item["CreatedAt"].(*types.AttributeValueMemberS).Value)
	if err != nil {
		return entities.Match{}, false, err
	}
	return entities.Match{
		Id:        matchId,
		Player1Id: output.Item["Player1Id"].(*types.AttributeValueMemberS).Value,
		Player2Id: output.Item["Player2Id"].(*types.AttributeValueMemberS).Value,
		Mode:      output.Item["Mode"].(*types.AttributeValueMemberS).Value,
		Server:    output.Item["Server"].(*types.AttributeValueMemberS).Value,
		CreatedAt: createdAt,
	}, true, nil
}

func notifyUser(userId string, data []byte) error {
	// Get the SNS topic ARN from environment variables
	topicARN := os.Getenv("SNS_TOPIC_ARN")
	if topicARN == "" {
		return fmt.Errorf("SNS_TOPIC_ARN environment variable is not set")
	}

	// Publish a message to the SNS topic
	publishInput := &sns.PublishInput{
		TopicArn: aws.String(topicARN),
		Message:  aws.String(string(data)), // Use the input message or customize it
		MessageAttributes: map[string]snsTypes.MessageAttributeValue{
			"userId": {
				DataType:    aws.String("String"),
				StringValue: aws.String(userId),
			},
		},
	}

	result, err := snsClient.Publish(ctx, publishInput)
	if err != nil {
		return fmt.Errorf("failed to publish message to SNS: %v", err)
	}

	log.Printf("Message published to SNS: %s\n", *result.MessageId)

	return nil
}

func main() {
	lambda.Start(handler)
}
