package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
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
	"github.com/chess-vn/slchess/internal/aws/auth"
	"github.com/chess-vn/slchess/internal/domains/dtos"
	"github.com/chess-vn/slchess/internal/domains/entities"
	"github.com/chess-vn/slchess/pkg/logging"
)

var (
	dynamoClient *dynamodb.Client
	ecsClient    *ecs.Client
	ec2Client    *ec2.Client

	clusterName     = os.Getenv("ECS_CLUSTER_NAME")
	serviceName     = os.Getenv("ECS_SERVICE_NAME")
	deploymentStage = os.Getenv("DEPLOYMENT_STAGE")

	ErrUserNotInMatch = fmt.Errorf("user not in match")
)

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	dynamoClient = dynamodb.NewFromConfig(cfg)
	ecsClient = ecs.NewFromConfig(cfg)
	ec2Client = ec2.NewFromConfig(cfg)
}

func handler(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	userId := auth.MustAuth(event.RequestContext.Authorizer)
	matchId := event.PathParameters["id"]

	checkAndStartServer(ctx)

	activeMatch, err := getActiveMatch(ctx, matchId)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest},
			fmt.Errorf("failed to get active match: %w", err)
	}
	if activeMatch.Player1.Id != userId && activeMatch.Player2.Id != userId {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest},
			fmt.Errorf("failed to restore match: %w", ErrUserNotInMatch)
	}

	var serverIp string
	for range 5 {
		serverIp, err = checkAndGetNewServerIp(ctx, clusterName, serviceName, activeMatch.Server)
		if err == nil {
			break
		}
		<-time.After(5 * time.Second)
	}
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError},
			fmt.Errorf("failed to get server ip: %w", err)
	}
	activeMatch.Server = serverIp

	activeMatchResp := dtos.ActiveMatchResponseFromEntity(activeMatch)
	activeMatchJson, err := json.Marshal(activeMatchResp)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError},
			fmt.Errorf("failed to marshal response: %w", err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(activeMatchJson),
	}, nil
}

func getActiveMatch(ctx context.Context, matchId string) (entities.ActiveMatch, error) {
	activeMatchOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("ActiveMatches"),
		Key: map[string]types.AttributeValue{
			"MatchId": &types.AttributeValueMemberS{Value: matchId},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return entities.ActiveMatch{}, err
	}

	var activeMatch entities.ActiveMatch
	if err := attributevalue.UnmarshalMap(activeMatchOutput.Item, &activeMatch); err != nil {
		return entities.ActiveMatch{}, err
	}
	return activeMatch, nil
}

func checkAndGetNewServerIp(ctx context.Context, clusterName, serviceName, targetPublicIp string) (string, error) {
	if deploymentStage == "dev" {
		return "SERVER_IP", nil
	}
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

	var newServerIp string
	for i, task := range describeTasksOutput.Tasks {
		for _, attachment := range task.Attachments {
			for _, detail := range attachment.Details {
				if *detail.Name == "networkInterfaceId" {
					eniID := *detail.Value

					eniOutput, err := ec2Client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
						NetworkInterfaceIds: []string{eniID},
					})
					if err != nil {
						return "", fmt.Errorf("failed to describe ENI: %w", err)
					}

					for _, eni := range eniOutput.NetworkInterfaces {
						if eni.Association != nil && eni.Association.PublicIp != nil {
							if *eni.Association.PublicIp == targetPublicIp {
								return targetPublicIp, nil
							}
							if i == 0 {
								newServerIp = *eni.Association.PublicIp
							}
						}
					}
				}
			}
		}
	}

	return newServerIp, nil
}

func checkAndStartServer(ctx context.Context) error {
	if deploymentStage == "dev" {
		return nil
	}
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

func main() {
	lambda.Start(handler)
}
