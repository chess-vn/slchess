package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snsTypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/bucket-sort/slchess/internal/domains/entities"
	"github.com/bucket-sort/slchess/pkg/utils"
)

var (
	dynamoClient *dynamodb.Client
	snsClient    *sns.Client
	ecsClient    *ecs.Client
	ec2Client    *ec2.Client
	ctx          context.Context
	timeLayout   string

	clusterName = os.Getenv("ECS_CLUSTER_NAME")
	serviceName = os.Getenv("ECS_SERVICE_NAME")
	region      = os.Getenv("AWS_REGION")

	ErrNoMatchFound       = errors.New("failed to matchmaking")
	ErrInvalidGameMode    = errors.New("invalid game mode")
	ErrServerNotAvailable = errors.New("server not available")
)

func init() {
	ctx = context.Background()
	timeLayout = "2006-01-02 15:04:05.999999999 -0700 MST"
	cfg, _ := config.LoadDefaultConfig(ctx)
	dynamoClient = dynamodb.NewFromConfig(cfg)
	snsClient = sns.NewFromConfig(cfg)
	ecsClient = ecs.NewFromConfig(cfg)
	ec2Client = ec2.NewFromConfig(cfg)
}

// Handle matchmaking requests
func handler(event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	v, exists := event.RequestContext.Authorizer["claims"]
	if !exists {
		return events.APIGatewayProxyResponse{
			StatusCode: 401,
			Body:       fmt.Sprintf("%v", event.RequestContext),
		}, nil
	}

	// Start game server beforehand if none available
	if err := checkAndStartServer(); err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "Failed to start game server",
		}, nil
	}

	claims := v.(map[string]interface{})
	userHandler := claims["cognito:username"].(string)

	var data map[string]interface{}
	json.Unmarshal([]byte(event.Body), &data)
	userRating := int(data["rating"].(float64))
	ratingLowerBound := int(data["lower_bound"].(float64))
	ratingUpperBound := int(data["upper_bound"].(float64))
	gameMode := data["game_mode"].(string)

	// Check if user already in a match
	match, exist, err := checkForActiveMatch(ctx, userHandler)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Matchmaking error: %v", err)}, nil
	}
	if exist {
		data, _ := json.Marshal(match)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusConflict, Body: string(data)}, nil
	}

	// Attempt matchmaking
	minRating := userRating + ratingLowerBound
	maxRating := userRating + ratingUpperBound
	log.Printf("Attempt matchmaking in rating range: %d - %d\n", minRating, maxRating)
	opponentHandlers, err := findOpponents(minRating, maxRating, userHandler, userRating, gameMode)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Matchmaking error: %v", err)}, nil
	}

	// If no match found, queue the player by caching the matchmaking ticket
	if len(opponentHandlers) == 0 {
		log.Println("No match found. Start queuing")
		return events.APIGatewayProxyResponse{StatusCode: http.StatusAccepted, Body: "Queued"}, nil
	}

	var errs []error
	// Try to create new match
	for _, opponentHandler := range opponentHandlers {
		match, err := createMatch(ctx, userHandler, opponentHandler, gameMode)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", opponentHandler, err))
			continue
		}
		log.Printf("Match found: %s: %s - %s\n", match.Id, match.Player1, match.Player2)
		matchJson, _ := json.Marshal(match)

		// Notify the opponent about the match
		err = notifyUser(userHandler, matchJson)
		if err != nil {
			log.Printf("Failed to notify user %s:%v\n", opponentHandler, err)
		}

		return events.APIGatewayProxyResponse{StatusCode: http.StatusCreated, Body: string(matchJson)}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: fmt.Sprintf("%v", errs)}, nil
}

// Matchmaking function using go-redis commands
func findOpponents(minRating, maxRating int, userId string, userRating int, gameMode string) ([]string, error) {
	output, err := dynamoClient.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String("MatchmakingTickets"),
		FilterExpression: aws.String("MinRating >= :minRating AND MaxRating <= :maxRating AND GameMode = :gameMode"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":minRating": &types.AttributeValueMemberN{Value: strconv.Itoa(minRating)},
			":maxRating": &types.AttributeValueMemberN{Value: strconv.Itoa(maxRating)},
			":gameMode":  &types.AttributeValueMemberS{Value: gameMode},
		},
	})
	if err != nil {
		return nil, err
	}

	var matches []string
	if output.Count > 0 {
		for _, item := range output.Items {
			if v, ok := item["UserHandler"].(*types.AttributeValueMemberS); ok && v.Value != userId {
				matches = append(matches, v.Value)
			}
		}
	} else {
		// No match found, add the user ticket to the queue
		_, err := dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String("MatchmakingTickets"),
			Item: map[string]types.AttributeValue{
				"UserHandler": &types.AttributeValueMemberS{Value: userId},
				"Rating":      &types.AttributeValueMemberN{Value: strconv.Itoa(userRating)},
				"MinRating":   &types.AttributeValueMemberN{Value: strconv.Itoa(minRating)},
				"MaxRating":   &types.AttributeValueMemberN{Value: strconv.Itoa(maxRating)},
				"GameMode":    &types.AttributeValueMemberS{Value: gameMode},
			},
		})
		if err != nil {
			return nil, err
		}
	}

	return matches, nil
}

func createMatch(ctx context.Context, userHandler, opponentHandler, gameMode string) (entities.Match, error) {
	// Try to wait till the server is running
	var (
		serverIp string
		err      error
	)
	for range 5 {
		serverIp, err = getServerIp(ctx, clusterName, serviceName)
		if err == nil {
			break
		}
		<-time.After(10 * time.Second)
	}
	if err != nil {
		return entities.Match{}, err
	}

	match := entities.Match{
		Id:        utils.GenerateUUID(),
		Player1:   userHandler,
		Player2:   opponentHandler,
		GameMode:  gameMode,
		Server:    serverIp,
		CreatedAt: time.Now(),
	}

	// Associate the players with created match to kind of mark them as matched
	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String("UserMatches"),
		ConditionExpression: aws.String("attribute_not_exists(UserHandler)"),
		Item: map[string]types.AttributeValue{
			"UserHandler": &types.AttributeValueMemberS{Value: opponentHandler},
			"MatchId":     &types.AttributeValueMemberS{Value: match.Id},
		},
	})
	if err != nil {
		var condCheckFailed *types.ConditionalCheckFailedException
		if errors.As(err, &condCheckFailed) {
			return entities.Match{}, fmt.Errorf("user already in a match: %s", opponentHandler)
		}
		return entities.Match{}, err
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String("UserMatches"),
		ConditionExpression: aws.String("attribute_not_exists(UserHandler)"),
		Item: map[string]types.AttributeValue{
			"UserHandler": &types.AttributeValueMemberS{Value: userHandler},
			"MatchId":     &types.AttributeValueMemberS{Value: match.Id},
		},
	})
	if err != nil {
		var condCheckFailed *types.ConditionalCheckFailedException
		if errors.As(err, &condCheckFailed) {
			return entities.Match{}, fmt.Errorf("user already in a match: %s", userHandler)
		}
		return entities.Match{}, err
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("ActiveMatches"),
		Item: map[string]types.AttributeValue{
			"MatchId":   &types.AttributeValueMemberS{Value: match.Id},
			"Player1":   &types.AttributeValueMemberS{Value: match.Player1},
			"Player2":   &types.AttributeValueMemberS{Value: match.Player2},
			"GameMode":  &types.AttributeValueMemberS{Value: match.GameMode},
			"Server":    &types.AttributeValueMemberS{Value: match.Server},
			"CreatedAt": &types.AttributeValueMemberS{Value: match.CreatedAt.Format(timeLayout)},
		},
	})
	if err != nil {
		return entities.Match{}, err
	}
	// Match created, remove opponent ticket from the queue
	_, err = dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("MatchmakingTickets"),
		Key: map[string]types.AttributeValue{
			"UserHandler": &types.AttributeValueMemberS{Value: opponentHandler},
		},
	})
	if err != nil {
		return entities.Match{}, err
	}
	return match, nil
}

func checkForActiveMatch(ctx context.Context, userHandler string) (entities.Match, bool, error) {
	output, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("UserMatches"),
		Key: map[string]types.AttributeValue{
			"UserHandler": &types.AttributeValueMemberS{
				Value: userHandler,
			},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return entities.Match{}, false, err
	}
	if output.Item == nil {
		return entities.Match{}, false, nil
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
		Player1:   output.Item["Player1"].(*types.AttributeValueMemberS).Value,
		Player2:   output.Item["Player2"].(*types.AttributeValueMemberS).Value,
		GameMode:  output.Item["GameMode"].(*types.AttributeValueMemberS).Value,
		Server:    output.Item["Server"].(*types.AttributeValueMemberS).Value,
		CreatedAt: createdAt,
	}, true, nil
}

func notifyUser(userHandler string, data []byte) error {
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
			"userHandler": {
				DataType:    aws.String("String"),
				StringValue: aws.String(userHandler),
			},
		},
	}

	result, err := snsClient.Publish(ctx, publishInput)
	if err != nil {
		return fmt.Errorf("failed to publish message to SNS: %v", err)
	}
	if result != nil {
		log.Printf("Message published to SNS: %s\n", *result.MessageId)
	}

	return nil
}

func getServerIp(ctx context.Context, clusterName, serviceName string) (string, error) {
	// List tasks in the cluster
	listTasksOutput, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       &clusterName,
		ServiceName:   &serviceName,
		DesiredStatus: "RUNNING",
	})
	if err != nil || len(listTasksOutput.TaskArns) == 0 {
		return "", fmt.Errorf("no running tasks found or error occurred: %v", err)
	}

	describeTasksOutput, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(clusterName),
		Tasks:   listTasksOutput.TaskArns,
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe ECS tasks: %w", err)
	}

	sort.Slice(describeTasksOutput.Tasks, func(i, j int) bool {
		return describeTasksOutput.Tasks[i].StartedAt.Before(*describeTasksOutput.Tasks[j].StartedAt)
	})

	var eniId string
	for _, detail := range describeTasksOutput.Tasks[0].Attachments[0].Details {
		if *detail.Name == "networkInterfaceId" {
			eniId = *detail.Value
			break
		}
	}

	if eniId == "" {
		return "", fmt.Errorf("ENI ID not found in task details")
	}

	// Get the public IP from EC2
	eniResp, err := ec2Client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		NetworkInterfaceIds: []string{eniId},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe network interface: %w", err)
	}

	if len(eniResp.NetworkInterfaces) == 0 || eniResp.NetworkInterfaces[0].Association == nil {
		return "", fmt.Errorf("no public IP found for ENI")
	}

	return *eniResp.NetworkInterfaces[0].Association.PublicIp, nil
}

func checkAndStartServer() error {
	ctx := context.TODO()

	// Load AWS Config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	ecsClient := ecs.NewFromConfig(cfg)

	// Check running task count
	listTasksOutput, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       aws.String(clusterName),
		ServiceName:   aws.String(serviceName),
		DesiredStatus: "RUNNING",
	})
	if err != nil {
		return fmt.Errorf("failed to list ECS tasks: %w", err)
	}

	// If no tasks are running, scale service to 1
	if len(listTasksOutput.TaskArns) == 0 {
		fmt.Println("No running tasks found. Scaling up ECS service...")

		_, err := ecsClient.UpdateService(ctx, &ecs.UpdateServiceInput{
			Cluster:      aws.String(clusterName),
			Service:      aws.String(serviceName),
			DesiredCount: aws.Int32(1),
		})
		if err != nil {
			return fmt.Errorf("failed to update ECS desired count: %w", err)
		}

		fmt.Println("ECS service scaled to 1 instance.")
	} else {
		fmt.Println("ECS service already running, no scaling needed.")
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
