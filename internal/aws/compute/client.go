package compute

import (
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

type Client struct {
	ecs *ecs.Client
	ec2 *ec2.Client
}

func NewClient(ecsClient *ecs.Client, ec2Client *ec2.Client) *Client {
	return &Client{
		ecs: ecsClient,
		ec2: ec2Client,
	}
}
