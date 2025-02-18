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
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snsTypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/chess-vn/slchess/internal/domains/entities"
	"github.com/chess-vn/slchess/pkg/logging"
	"github.com/chess-vn/slchess/pkg/utils"
	"go.uber.org/zap"
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
	userHandler := mustAuth(event.RequestContext.Authorizer)

	// Start game server beforehand if none available
	if err := checkAndStartServer(ctx); err != nil {
		logging.Error("Failed to start game server", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	// Extract and validate matchmaking ticket
	var ticket entities.MatchmakingTicket
	if err := json.Unmarshal([]byte(event.Body), &ticket); err != nil {
		logging.Error("Failed to extract ticket", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest}, nil
	}
	if err := ticket.Validate(); err != nil {
		logging.Error("Invalid ticket", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest}, nil
	}
	ticket.UserHandler = userHandler

	// Check if user already in a match
	match, exist, err := checkForActiveMatch(ctx, userHandler)
	if err != nil {
		logging.Error("Failed to check for active match", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}
	if exist {
		data, _ := json.Marshal(match)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(data)}, nil
	}

	// Attempt matchmaking
	logging.Info("Attempt matchmaking", zap.Float64("minRating", ticket.MinRating), zap.Float64("maxRating", ticket.MaxRating))
	opponents, err := findOpponents(ticket)
	if err != nil {
		logging.Error("Failed to find opponents", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	// If no match found, queue the player by caching the matchmaking ticket
	if len(opponents) == 0 {
		logging.Info("No match found. Start queuing")
		return events.APIGatewayProxyResponse{StatusCode: http.StatusAccepted, Body: "Queued"}, nil
	}

	user := entities.Player{
		Handler: ticket.UserHandler,
		Rating:  ticket.Rating,
		RD:      ticket.RD,
	}

	// Try to create new match
	for _, opponent := range opponents {
		match, err := createMatch(ctx, user, opponent, ticket.GameMode)
		if err != nil {
			logging.Error("Failed to create match",
				zap.String("user", user.Handler),
				zap.String("opponent", opponent.Handler),
				zap.Error(err),
			)
			continue
		}
		logging.Info("Match found",
			zap.String("matchId", match.MatchId),
			zap.String("player1", match.Player1.Handler),
			zap.String("player2", match.Player2.Handler),
		)
		matchJson, _ := json.Marshal(match)

		// Notify the opponent about the match
		err = notifyUser(userHandler, matchJson)
		if err != nil {
			logging.Error("Failed to notify user",
				zap.String("userHandler", opponent.Handler),
				zap.Error(err),
			)
		}

		return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(matchJson)}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
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
	userHandler, ok := claims["cognito:username"].(string)
	if !ok {
		panic("invalid user handler")
	}
	return userHandler
}

// Matchmaking function using go-redis commands
func findOpponents(ticket entities.MatchmakingTicket) ([]entities.Player, error) {
	output, err := dynamoClient.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String("MatchmakingTickets"),
		FilterExpression: aws.String("MinRating >= :minRating AND MaxRating <= :maxRating AND GameMode = :gameMode"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":minRating": &types.AttributeValueMemberN{Value: strconv.Itoa(int(ticket.MinRating))},
			":maxRating": &types.AttributeValueMemberN{Value: strconv.Itoa(int(ticket.MaxRating))},
			":gameMode":  &types.AttributeValueMemberS{Value: ticket.GameMode},
		},
	})
	if err != nil {
		return nil, err
	}

	var tickets []entities.MatchmakingTicket
	err = attributevalue.UnmarshalListOfMaps(output.Items, &tickets)
	if err != nil {
		return nil, err
	}

	var opponents []entities.Player
	if output.Count > 0 {
		for _, opTicket := range tickets {
			if opTicket.UserHandler == ticket.UserHandler {
				continue
			}
			opponents = append(opponents, entities.Player{
				Handler: opTicket.UserHandler,
				Rating:  opTicket.Rating,
				RD:      opTicket.RD,
			})
		}
	} else {
		// No match found, add the user ticket to the queue
		ticketAv, _ := attributevalue.MarshalMap(ticket)
		_, err := dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String("MatchmakingTickets"),
			Item:      ticketAv,
		})
		if err != nil {
			return nil, err
		}
	}

	return opponents, nil
}

func createMatch(ctx context.Context, user, opponent entities.Player, gameMode string) (entities.ActiveMatch, error) {
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
		<-time.After(5 * time.Second)
	}
	if err != nil {
		return entities.ActiveMatch{}, err
	}

	match := entities.ActiveMatch{
		MatchId:   utils.GenerateUUID(),
		Player1:   user,
		Player2:   opponent,
		GameMode:  gameMode,
		Server:    serverIp,
		CreatedAt: time.Now(),
	}

	// Associate the players with created match to kind of mark them as matched
	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String("UserMatches"),
		ConditionExpression: aws.String("attribute_not_exists(UserHandler)"),
		Item: map[string]types.AttributeValue{
			"UserHandler": &types.AttributeValueMemberS{Value: opponent.Handler},
			"MatchId":     &types.AttributeValueMemberS{Value: match.MatchId},
		},
	})
	if err != nil {
		var condCheckFailed *types.ConditionalCheckFailedException
		if errors.As(err, &condCheckFailed) {
			return entities.ActiveMatch{}, fmt.Errorf("user already in a match: %s", opponent.Handler)
		}
		return entities.ActiveMatch{}, err
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String("UserMatches"),
		ConditionExpression: aws.String("attribute_not_exists(UserHandler)"),
		Item: map[string]types.AttributeValue{
			"UserHandler": &types.AttributeValueMemberS{Value: user.Handler},
			"MatchId":     &types.AttributeValueMemberS{Value: match.MatchId},
		},
	})
	if err != nil {
		var condCheckFailed *types.ConditionalCheckFailedException
		if errors.As(err, &condCheckFailed) {
			return entities.ActiveMatch{}, fmt.Errorf("user already in a match: %s", user.Handler)
		}
		return entities.ActiveMatch{}, err
	}

	match.Player1.RatingChanges = calculateRatingChange(match.Player1, match.Player2.Rating)
	match.Player2.RatingChanges = calculateRatingChange(match.Player2, match.Player1.Rating)
	matchAv, err := attributevalue.MarshalMap(match)
	if err != nil {
		log.Fatalf("Failed to save match state: %v", err)
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("ActiveMatches"),
		Item:      matchAv,
	})
	if err != nil {
		return entities.ActiveMatch{}, err
	}
	// Match created, remove opponent ticket from the queue
	_, err = dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("MatchmakingTickets"),
		Key: map[string]types.AttributeValue{
			"UserHandler": &types.AttributeValueMemberS{Value: opponent.Handler},
		},
	})
	if err != nil {
		return entities.ActiveMatch{}, err
	}
	return match, nil
}

func checkForActiveMatch(ctx context.Context, userHandler string) (entities.ActiveMatch, bool, error) {
	userMatchOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("UserMatches"),
		Key: map[string]types.AttributeValue{
			"UserHandler": &types.AttributeValueMemberS{
				Value: userHandler,
			},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return entities.ActiveMatch{}, false, err
	}
	if userMatchOutput.Item == nil {
		return entities.ActiveMatch{}, false, nil
	}

	var userMatch entities.UserMatch
	if err := attributevalue.UnmarshalMap(userMatchOutput.Item, &userMatch); err != nil {
		return entities.ActiveMatch{}, false, err
	}

	activeMatchOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("ActiveMatches"),
		Key: map[string]types.AttributeValue{
			"MatchId": &types.AttributeValueMemberS{Value: userMatch.MatchId},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return entities.ActiveMatch{}, false, err
	}
	if activeMatchOutput.Item == nil {
		return entities.ActiveMatch{}, false, nil
	}

	var activeMatch entities.ActiveMatch
	if err := attributevalue.UnmarshalMap(userMatchOutput.Item, activeMatch); err != nil {
		return entities.ActiveMatch{}, false, err
	}
	return activeMatch, true, nil
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
		logging.Info("Message published to SNS: %s\n", zap.String("id", *result.MessageId))
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

func checkAndStartServer(ctx context.Context) error {
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
		logging.Info("No running tasks found. Scaling up ECS service...")

		_, err := ecsClient.UpdateService(ctx, &ecs.UpdateServiceInput{
			Cluster:      aws.String(clusterName),
			Service:      aws.String(serviceName),
			DesiredCount: aws.Int32(1),
		})
		if err != nil {
			return fmt.Errorf("failed to update ECS desired count: %w", err)
		}
		logging.Info("ECS service scaled to 1 instance.")
	} else {
		logging.Info("ECS service already running, no scaling needed.")
	}

	return nil
}

func calculateRatingChange(player entities.Player, opRating float64) []float64 {
	return []float64{10.0, 0, -10.0}
}

func main() {
	lambda.Start(handler)
}
