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
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/chess-vn/slchess/internal/domains/dtos"
	"github.com/chess-vn/slchess/internal/domains/entities"
	"github.com/chess-vn/slchess/pkg/logging"
	"github.com/chess-vn/slchess/pkg/utils"
	"go.uber.org/zap"
)

var (
	dynamoClient *dynamodb.Client
	ecsClient    *ecs.Client
	ec2Client    *ec2.Client

	clusterName       = os.Getenv("ECS_CLUSTER_NAME")
	serviceName       = os.Getenv("ECS_SERVICE_NAME")
	region            = os.Getenv("AWS_REGION")
	websocketApiId    = os.Getenv("WEBSOCKET_API_ID")
	websocketApiStage = os.Getenv("WEBSOCKET_API_STAGE")

	ErrNoMatchFound       = errors.New("failed to matchmaking")
	ErrInvalidGameMode    = errors.New("invalid game mode")
	ErrServerNotAvailable = errors.New("server not available")

	timeLayout = "2006-01-02 15:04:05.999999999 -0700 MST"
)

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
	ecsClient = ecs.NewFromConfig(cfg)
	ec2Client = ec2.NewFromConfig(cfg)
}

// Handle matchmaking requests
func handler(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	userId := mustAuth(event.RequestContext.Authorizer)

	// Start game server beforehand if none available
	if err := checkAndStartServer(ctx); err != nil {
		logging.Error("Failed to start game server", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	// Extract and validate matchmaking ticket
	var matchmakingReq dtos.MatchmakingRequest
	if err := json.Unmarshal([]byte(event.Body), &matchmakingReq); err != nil {
		logging.Error("Failed to validate request", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest}, nil
	}
	userRating, err := getUserRating(ctx, userId)
	if err != nil {
		logging.Error("Failed to get user rating", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}
	if userRating.Rating < matchmakingReq.MinRating || userRating.Rating > matchmakingReq.MaxRating {
		logging.Error("Invalid matchmaking ticket", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest}, nil
	}
	ticket := dtos.MatchmakingRequestToEntity(userId, matchmakingReq)

	// Check if user already in a activeMatch
	activeMatch, exist, err := checkForActiveMatch(ctx, userId)
	if err != nil {
		logging.Error("Failed to check for active match", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}
	if exist {
		data, _ := json.Marshal(activeMatch)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(data)}, nil
	}

	// Attempt matchmaking
	logging.Info("Attempt matchmaking", zap.Float64("minRating", ticket.MinRating), zap.Float64("maxRating", ticket.MaxRating))
	opponentIds, err := findOpponents(ctx, ticket)
	if err != nil {
		logging.Error("Failed to find opponents", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	// If no match found, queue the player by caching the matchmaking ticket
	if len(opponentIds) == 0 {
		logging.Info("No match found. Start queuing")
		return events.APIGatewayProxyResponse{StatusCode: http.StatusAccepted, Body: "Queued"}, nil
	}

	// Try to create new match
	for _, opponentId := range opponentIds {
		match, err := createMatch(ctx, userRating, opponentId, ticket.GameMode)
		if err != nil {
			logging.Error("Failed to create match",
				zap.String("user", userId),
				zap.String("opponent", opponentId),
				zap.Error(err),
			)
			continue
		}
		logging.Info("Match found",
			zap.String("matchId", match.MatchId),
			zap.String("player1", match.Player1.Id),
			zap.String("player2", match.Player2.Id),
		)
		matchResp := dtos.ActiveMatchResponseFromEntity(match)
		matchRespJson, _ := json.Marshal(matchResp)

		// Notify the opponent about the match
		err = notifyQueueingUser(ctx, opponentId, matchRespJson)
		if err != nil {
			logging.Error("Failed to notify queueing user",
				zap.String("userId", opponentId),
				zap.Error(err),
			)
		}

		return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(matchRespJson)}, nil
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
	userId, ok := claims["sub"].(string)
	if !ok {
		panic("invalid sub")
	}
	return userId
}

// Matchmaking function using go-redis commands
func findOpponents(ctx context.Context, ticket entities.MatchmakingTicket) ([]string, error) {
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

	var opponentIds []string
	if output.Count > 0 {
		for _, opTicket := range tickets {
			if opTicket.UserId == ticket.UserId {
				continue
			}
			opponentIds = append(opponentIds, opTicket.UserId)
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

	return opponentIds, nil
}

func createMatch(ctx context.Context, userRating entities.UserRating, opponentId, gameMode string) (entities.ActiveMatch, error) {
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
		GameMode:  gameMode,
		Server:    serverIp,
		CreatedAt: time.Now(),
	}

	// Associate the players with created match to kind of mark them as matched
	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String("UserMatches"),
		ConditionExpression: aws.String("attribute_not_exists(UserId)"),
		Item: map[string]types.AttributeValue{
			"UserId":  &types.AttributeValueMemberS{Value: opponentId},
			"MatchId": &types.AttributeValueMemberS{Value: match.MatchId},
		},
	})
	if err != nil {
		var condCheckFailed *types.ConditionalCheckFailedException
		if errors.As(err, &condCheckFailed) {
			return entities.ActiveMatch{}, fmt.Errorf("user already in a match: %s", opponentId)
		}
		return entities.ActiveMatch{}, err
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String("UserMatches"),
		ConditionExpression: aws.String("attribute_not_exists(UserId)"),
		Item: map[string]types.AttributeValue{
			"UserId":  &types.AttributeValueMemberS{Value: userRating.UserId},
			"MatchId": &types.AttributeValueMemberS{Value: match.MatchId},
		},
	})
	if err != nil {
		var condCheckFailed *types.ConditionalCheckFailedException
		if errors.As(err, &condCheckFailed) {
			return entities.ActiveMatch{}, fmt.Errorf("user already in a match: %s", userRating.UserId)
		}
		return entities.ActiveMatch{}, err
	}

	// Pre-calculate players' rating in each possible outcome
	opponentRating, err := getUserRating(ctx, opponentId)
	if err != nil {
		return entities.ActiveMatch{}, err
	}

	newUserRatings, newUserRDs, err := calculateNewRatings(ctx, userRating, opponentRating)
	if err != nil {
		return entities.ActiveMatch{}, err
	}
	match.Player1 = entities.Player{
		Id:         userRating.UserId,
		Rating:     userRating.Rating,
		RD:         userRating.RD,
		NewRatings: newUserRatings,
		NewRDs:     newUserRDs,
	}

	newOpponentRatings, newOpponentRatingsRDs, err := calculateNewRatings(ctx, opponentRating, userRating)
	if err != nil {
		return entities.ActiveMatch{}, err
	}
	match.Player2 = entities.Player{
		Id:         opponentRating.UserId,
		Rating:     opponentRating.Rating,
		RD:         opponentRating.RD,
		NewRatings: newOpponentRatings,
		NewRDs:     newOpponentRatingsRDs,
	}

	// Save match information
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
			"UserId": &types.AttributeValueMemberS{Value: opponentId},
		},
	})
	if err != nil {
		return entities.ActiveMatch{}, err
	}
	return match, nil
}

func checkForActiveMatch(ctx context.Context, userId string) (entities.ActiveMatch, bool, error) {
	userMatchOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("UserMatches"),
		Key: map[string]types.AttributeValue{
			"UserId": &types.AttributeValueMemberS{
				Value: userId,
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

func notifyQueueingUser(ctx context.Context, userId string, matchJson []byte) error {
	cfg, _ := config.LoadDefaultConfig(ctx)

	// Get user ID from DynamoDB
	connectionOutput, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String("Connections"),
		IndexName:              aws.String("UserIdIndex"),
		KeyConditionExpression: aws.String("UserId = :userId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userId": &types.AttributeValueMemberS{Value: userId},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return err
	}
	if len(connectionOutput.Items) > 0 {
		var connection entities.Connection
		if err := attributevalue.UnmarshalMap(connectionOutput.Items[0], &connection); err != nil {
			return err
		}
		apiEndpoint := fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/%s", websocketApiId, region, websocketApiStage)
		apiGatewayClient := apigatewaymanagementapi.New(apigatewaymanagementapi.Options{
			BaseEndpoint: aws.String(apiEndpoint),
			Region:       region,
			Credentials:  cfg.Credentials,
		})
		_, err := apiGatewayClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: &connection.Id,
			Data:         matchJson,
		})
		if err != nil {
			return err
		}
		logging.Info("User notified", zap.String("userId", userId))
	} else {
		logging.Info("User not connected", zap.String("userId", userId))
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

func getUserRating(ctx context.Context, userId string) (entities.UserRating, error) {
	userRatingOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("UserRatings"),
		Key: map[string]types.AttributeValue{
			"UserId": &types.AttributeValueMemberS{Value: userId},
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

func main() {
	lambda.Start(handler)
}
