package compute

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/chess-vn/slchess/internal/domains/dtos"
	"github.com/chess-vn/slchess/pkg/logging"
)

var (
	ErrNoServerAvailable   = fmt.Errorf("no server available")
	ErrUnknownServerStatus = fmt.Errorf("unknown server status")
)

type TaskMetadata struct {
	TaskArn     string `json:"TaskARN"`
	ClusterName string `json:"Cluster"`
}

func (client *Client) GetAvailableServerIp(
	ctx context.Context,
	clusterName,
	serviceName string,
) (string, int, error) {
	// List tasks in the cluster
	listTasksOutput, err := client.ecs.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       &clusterName,
		ServiceName:   &serviceName,
		DesiredStatus: "RUNNING",
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to list tasks: %w", err)
	}
	if len(listTasksOutput.TaskArns) == 0 {
		return "", 0, ErrNoServerAvailable
	}

	describeTasksOutput, err := client.ecs.DescribeTasks(
		ctx,
		&ecs.DescribeTasksInput{
			Cluster: aws.String(clusterName),
			Tasks:   listTasksOutput.TaskArns,
		},
	)
	if err != nil || describeTasksOutput == nil {
		return "", 0, fmt.Errorf("failed to describe ECS tasks: %w", err)
	}

	pendingCount := 0
	for _, task := range describeTasksOutput.Tasks {
		if task.StartedAt == nil {
			pendingCount += 1
		}
	}

	// Sort game server by start time in descending order
	sort.Slice(describeTasksOutput.Tasks, func(i, j int) bool {
		return describeTasksOutput.Tasks[i].StartedAt.After(*describeTasksOutput.Tasks[j].StartedAt)
	})

	for _, task := range describeTasksOutput.Tasks {
		for _, attachment := range task.Attachments {
			for _, detail := range attachment.Details {
				if *detail.Name == "networkInterfaceId" {
					eniID := *detail.Value

					eniOutput, err := client.ec2.DescribeNetworkInterfaces(
						ctx,
						&ec2.DescribeNetworkInterfacesInput{
							NetworkInterfaceIds: []string{eniID},
						},
					)
					if err != nil {
						return "", 0, fmt.Errorf("failed to describe ENI: %w", err)
					}

					for _, eni := range eniOutput.NetworkInterfaces {
						if eni.Association != nil && eni.Association.PublicIp != nil {
							serverIp := *eni.Association.PublicIp
							status, err := client.GetServerStatus(serverIp, 7202)
							if err != nil {
								return "", 0, fmt.Errorf("failed to get server status: %w", err)
							}

							if status.CanAccept {
								return serverIp, 0, nil
							}
						}
					}
				}
			}
		}
	}

	return "", pendingCount, ErrNoServerAvailable
}

func (client *Client) CheckAndGetNewServerIp(
	ctx context.Context,
	clusterName,
	serviceName,
	targetPublicIp string,
) (string, error) {
	// List tasks in the cluster
	listTasksOutput, err := client.ecs.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       &clusterName,
		ServiceName:   &serviceName,
		DesiredStatus: "RUNNING",
	})
	if err != nil || len(listTasksOutput.TaskArns) == 0 {
		return "", fmt.Errorf("no running tasks found or error occurred: %v", err)
	}

	describeTasksOutput, err := client.ecs.DescribeTasks(
		ctx,
		&ecs.DescribeTasksInput{
			Cluster: aws.String(clusterName),
			Tasks:   listTasksOutput.TaskArns,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to describe ECS tasks: %w", err)
	}

	sort.Slice(describeTasksOutput.Tasks, func(i, j int) bool {
		return describeTasksOutput.Tasks[i].StartedAt.After(*describeTasksOutput.Tasks[j].StartedAt)
	})

	var serverIps []string
	for _, task := range describeTasksOutput.Tasks {
		for _, attachment := range task.Attachments {
			for _, detail := range attachment.Details {
				if *detail.Name == "networkInterfaceId" {
					eniID := *detail.Value

					eniOutput, err := client.ec2.DescribeNetworkInterfaces(
						ctx,
						&ec2.DescribeNetworkInterfacesInput{
							NetworkInterfaceIds: []string{eniID},
						},
					)
					if err != nil {
						return "", fmt.Errorf("failed to describe ENI: %w", err)
					}

					for _, eni := range eniOutput.NetworkInterfaces {
						if eni.Association != nil && eni.Association.PublicIp != nil {
							serverIp := *eni.Association.PublicIp
							if serverIp == targetPublicIp {
								return targetPublicIp, nil
							}
							serverIps = append(serverIps, serverIp)
						}
					}
				}
			}
		}
	}

	for _, serverIp := range serverIps {
		status, err := client.GetServerStatus(serverIp, 7202)
		if err != nil {
			return "", fmt.Errorf("failed to get server status: %w", err)
		}

		if status.CanAccept {
			return serverIp, nil
		}
	}

	return "", ErrNoServerAvailable
}

func (client *Client) CheckAndStartNewTask(
	ctx context.Context,
	clusterName,
	serviceName string,
) error {
	// Check running task count
	listTasksOutput, err := client.ecs.ListTasks(ctx, &ecs.ListTasksInput{
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

		_, err := client.ecs.UpdateService(ctx, &ecs.UpdateServiceInput{
			Cluster:      aws.String(clusterName),
			Service:      aws.String(serviceName),
			DesiredCount: aws.Int32(1),
		})
		if err != nil {
			return fmt.Errorf("failed to update ECS desired count: %w", err)
		}
	}

	return nil
}

func (client *Client) StartNewTask(
	ctx context.Context,
	clusterName,
	serviceName string,
) error {
	listTasksOutput, err := client.ecs.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       aws.String(clusterName),
		ServiceName:   aws.String(serviceName),
		DesiredStatus: "RUNNING",
	})
	if err != nil {
		return fmt.Errorf("failed to list ECS tasks: %w", err)
	}

	_, err = client.ecs.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      aws.String(clusterName),
		Service:      aws.String(serviceName),
		DesiredCount: aws.Int32(int32(len(listTasksOutput.TaskArns)) + 1),
	})
	if err != nil {
		return fmt.Errorf("failed to update ECS desired count: %w", err)
	}

	return nil
}

func (client *Client) UpdateServerProtection(
	ctx context.Context,
	enabled bool,
) error {
	if client.cfg.ClusterName == nil || client.cfg.TaskArn == nil {
		return fmt.Errorf("missing task metadata")
	}
	_, err := client.ecs.UpdateTaskProtection(ctx, &ecs.UpdateTaskProtectionInput{
		Cluster:           client.cfg.ClusterName,
		Tasks:             []string{*client.cfg.TaskArn},
		ProtectionEnabled: enabled,
	})
	if err != nil {
		return fmt.Errorf("failed to update task protection: %w", err)
	}
	return nil
}

func (client *Client) GetServerStatus(ip string, host int) (dtos.ServerStatusResponse, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("http://%s:%d/status", ip, host),
		nil,
	)
	if err != nil {
		return dtos.ServerStatusResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.http.Do(req)
	if err != nil {
		return dtos.ServerStatusResponse{}, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return dtos.ServerStatusResponse{}, ErrUnknownServerStatus
	}
	var status dtos.ServerStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return dtos.ServerStatusResponse{}, fmt.Errorf("failed to decode body: %w", err)
	}
	return status, nil
}
