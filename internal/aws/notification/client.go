package notification

import "github.com/aws/aws-sdk-go-v2/service/sns"

type Client struct {
	sns *sns.Client
	cfg config
}

type config struct{}

func NewClient(snsClient *sns.Client) *Client {
	return &Client{
		sns: snsClient,
		cfg: loadConfig(),
	}
}

func loadConfig() config {
	var cfg config

	return cfg
}
