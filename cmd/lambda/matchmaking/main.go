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
	"github.com/chess-vn/slchess/pkg/utils"
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
	userRating := data["rating"].(float64)
	ratingLowerBound := data["lower_bound"].(float64)
	ratingUpperBound := data["upper_bound"].(float64)
	userRd := data["rd"].(float64)
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
	log.Printf("Attempt matchmaking in rating range: %f - %f\n", minRating, maxRating)
	opponents, err := findOpponents(entities.MatchmakingTicket{
		UserHandler: userHandler,
		Rating:      userRating,
		MinRating:   minRating,
		MaxRating:   maxRating,
		RD:          userRd,
		GameMode:    gameMode,
	})
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Matchmaking error: %v", err)}, nil
	}

	// If no match found, queue the player by caching the matchmaking ticket
	if len(opponents) == 0 {
		log.Println("No match found. Start queuing")
		return events.APIGatewayProxyResponse{StatusCode: http.StatusAccepted, Body: "Queued"}, nil
	}

	user := entities.Player{
		Handler: userHandler,
		Rating:  userRating,
		RD:      userRd,
	}
	var errs []error
	// Try to create new match
	for _, opponent := range opponents {
		match, err := createMatch(ctx, user, opponent, gameMode)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", opponent.Handler, err))
			continue
		}
		log.Printf("Match found: %s: %s - %s\n", match.MatchId, match.Player1.Handler, match.Player2.Handler)
		matchJson, _ := json.Marshal(match)

		// Notify the opponent about the match
		err = notifyUser(userHandler, matchJson)
		if err != nil {
			log.Printf("Failed to notify user %s:%v\n", opponent.Handler, err)
		}

		return events.APIGatewayProxyResponse{StatusCode: http.StatusCreated, Body: string(matchJson)}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: fmt.Sprintf("%v", errs)}, nil
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
		<-time.After(10 * time.Second)
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
		return entities.ActiveMatch{}, false, err
	}
	if output.Item == nil {
		return entities.ActiveMatch{}, false, nil
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
		return entities.ActiveMatch{}, false, err
	}
	if output.Item == nil {
		return entities.ActiveMatch{}, false, nil
	}

	createdAt, err := time.Parse(timeLayout, output.Item["CreatedAt"].(*types.AttributeValueMemberS).Value)
	if err != nil {
		return entities.ActiveMatch{}, false, err
	}
	return entities.ActiveMatch{
		MatchId: matchId,
		Player1: entities.Player{
			Handler: output.Item["Player1"].(*types.AttributeValueMemberS).Value,
		},
		Player2: entities.Player{
			Handler: output.Item["Player2"].(*types.AttributeValueMemberS).Value,
		},
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

func calculateRatingChange(player entities.Player, opRating float64) []float64 {
	return []float64{10.0, 0, -10.0}
}

func main() {
	lambda.Start(handler)
}
