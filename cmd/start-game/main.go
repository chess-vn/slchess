package main

import (
	"github.com/yelaco/graduation-thesis/internal/handlers"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handlers.StartGameHandler)
}
