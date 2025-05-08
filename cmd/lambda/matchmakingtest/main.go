package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/chess-vn/slchess/internal/aws/compute"
	"github.com/chess-vn/slchess/internal/aws/storage"
	"github.com/chess-vn/slchess/internal/domains/dtos"
	"github.com/chess-vn/slchess/internal/domains/entities"
	"github.com/chess-vn/slchess/pkg/utils"
)

var (
	storageClient    *storage.Client
	computeClient    *compute.Client
	apigatewayClient *apigatewaymanagementapi.Client

	clusterName       = os.Getenv("SERVER_CLUSTER_NAME")
	serviceName       = os.Getenv("SERVER_SERVICE_NAME")
	region            = os.Getenv("AWS_REGION")
	websocketApiId    = os.Getenv("WEBSOCKET_API_ID")
	websocketApiStage = os.Getenv("WEBSOCKET_API_STAGE")
	deploymentStage   = os.Getenv("DEPLOYMENT_STAGE")

	ErrNoMatchFound           = errors.New("failed to matchmaking")
	ErrInvalidGameMode        = errors.New("invalid game mode")
	ErrServertestNotAvailable = errors.New("servertest not available")

	timeLayout  = "2006-01-02 15:04:05.999999999 -0700 MST"
	apiEndpoint = fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/%s", websocketApiId, region, websocketApiStage)
)

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	storageClient = storage.NewClient(dynamodb.NewFromConfig(cfg))
	computeClient = compute.NewClient(
		ecs.NewFromConfig(cfg),
		ec2.NewFromConfig(cfg),
		nil,
	)
	apigatewayClient = apigatewaymanagementapi.New(apigatewaymanagementapi.Options{
		BaseEndpoint: aws.String(apiEndpoint),
		Region:       region,
		Credentials:  cfg.Credentials,
	})
}

func handler(
	ctx context.Context,
	event events.APIGatewayProxyRequest,
) (
	events.APIGatewayProxyResponse,
	error,
) {
	userId := event.QueryStringParameters["userId"]

	// Extract and validate matchmaking ticket
	var matchmakingReq dtos.MatchmakingRequest
	err := json.Unmarshal([]byte(event.Body), &matchmakingReq)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
		}, fmt.Errorf("failed to validate request: %w", err)
	}

	userRating := entities.UserRating{
		UserId:       userId,
		PartitionKey: "UserRatings",
		Rating:       1200,
		RD:           200,
	}
	ticket := dtos.MatchmakingRequestToEntity(userRating, matchmakingReq)
	if err := ticket.Validate(); err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
		}, fmt.Errorf("invalid ticket: %w", err)
	}

	// Check if user already in a activeMatch
	activeMatch, err := storageClient.CheckForActiveMatch(ctx, userId)
	if err != nil {
		if !errors.Is(err, storage.ErrUserMatchNotFound) &&
			!errors.Is(err, storage.ErrActiveMatchNotFound) {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
			}, fmt.Errorf("failed to check for active match: %w", err)
		}
	} else {
		matchResp := dtos.ActiveMatchResponseFromEntity(activeMatch)
		matchRespJson, _ := json.Marshal(matchResp)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Body:       string(matchRespJson),
		}, nil
	}

	// Attempt matchmaking
	opponentIds, err := findOpponents(ctx, ticket)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to find opponents: %w", err)
	}

	// If no match found, queue the player by caching the matchmaking ticket
	if len(opponentIds) == 0 {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusAccepted,
			Body:       "Queued",
		}, nil
	}

	// Retrieve ip address of an available server
	var (
		serverIp     string
		pendingCount int
	)
	for range 5 {
		serverIp, pendingCount, err = computeClient.GetAvailableServerIp(ctx, clusterName, serviceName)
		if err == nil {
			break
		}
		if errors.Is(err, compute.ErrNoServerAvailable) && pendingCount == 0 {
			computeClient.StartNewTask(ctx, clusterName, serviceName)
		}
		time.Sleep(5 + time.Duration(rand.IntN(5))*time.Second)
	}
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to get server ip: %w", err)
	}

	// Try to create new match
	for _, opponentId := range opponentIds {
		match, err := createMatch(
			ctx,
			userId,
			opponentId,
			ticket.GameMode,
			serverIp,
		)
		if err != nil {
			continue
		}
		matchResp := dtos.ActiveMatchResponseFromEntity(match)
		matchRespJson, err := json.Marshal(matchResp)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
			}, fmt.Errorf("failed to marshal response: %w", err)
		}

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Body:       string(matchRespJson),
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusInternalServerError,
	}, nil
}

// Matchmaking function using go-redis commands
func findOpponents(
	ctx context.Context,
	ticket entities.MatchmakingTicket,
) (
	[]string,
	error,
) {
	tickets, err := storageClient.ScanMatchmakingTickets(ctx, ticket)
	if err != nil {
		return nil, fmt.Errorf("failed to scan matchmaking tickets: %w", err)
	}

	var opponentIds []string
	if len(tickets) > 0 {
		for _, opTicket := range tickets {
			if opTicket.UserId == ticket.UserId {
				continue
			}
			opponentIds = append(opponentIds, opTicket.UserId)
		}
	} else {
		// No match found, add the user ticket to the queue
		storageClient.PutMatchmakingTickets(ctx, ticket)
	}

	return opponentIds, nil
}

func createMatch(
	ctx context.Context,
	userId,
	opponentId,
	gameMode string,
	serverIp string,
) (
	entities.ActiveMatch,
	error,
) {
	match := entities.ActiveMatch{
		MatchId:        utils.GenerateUUID(),
		ConversationId: utils.GenerateUUID(),
		PartitionKey:   "ActiveMatches",
		GameMode:       gameMode,
		Server:         serverIp,
		CreatedAt:      time.Now(),
	}

	err := storageClient.PutUserMatch(ctx, entities.UserMatch{
		UserId:  opponentId,
		MatchId: match.MatchId,
	})
	if err != nil {
		return entities.ActiveMatch{}, err
	}

	err = storageClient.PutUserMatch(ctx, entities.UserMatch{
		UserId:  userId,
		MatchId: match.MatchId,
	})
	if err != nil {
		return entities.ActiveMatch{}, err
	}

	storageClient.PutActiveMatch(ctx, match)

	// Match created, remove opponent ticket from the queue
	err = storageClient.DeleteMatchmakingTicket(ctx, opponentId)
	if err != nil {
		return entities.ActiveMatch{},
			fmt.Errorf(
				"failed to delete matchmaking ticket: [userId: %s] %w",
				opponentId,
				err,
			)
	}

	return match, nil
}

func notifyQueueingUser(ctx context.Context, userId string, data []byte) error {
	// Get user ID from DynamoDB
	connection, err := storageClient.GetConnectionByUserId(ctx, userId)
	if err != nil {
		if errors.Is(err, storage.ErrConnectionNotFound) {
			return nil
		}
		return fmt.Errorf("failed to get connection: %w", err)
	}

	_, err = apigatewayClient.PostToConnection(
		ctx,
		&apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: aws.String(connection.Id),
			Data:         data,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to post to connect: %w", err)
	}

	_, err = apigatewayClient.DeleteConnection(
		ctx,
		&apigatewaymanagementapi.DeleteConnectionInput{
			ConnectionId: aws.String(connection.Id),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to delete connection: %w", err)
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
