package storage

import "github.com/aws/aws-sdk-go-v2/service/dynamodb"

type Client struct {
	dynamodb *dynamodb.Client
}

func NewClient(dynamoClient *dynamodb.Client) *Client {
	return &Client{
		dynamodb: dynamoClient,
	}
}
