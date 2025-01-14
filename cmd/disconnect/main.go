package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/yelaco/graduation-thesis/internal/handlers"
)

func main() {
	lambda.Start(handlers.DisconnectHandler)
}
