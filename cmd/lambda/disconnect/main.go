package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	handlers "github.com/bucket-sort/slchess/internal/lambda"
)

func main() {
	lambda.Start(handlers.DisconnectHandler)
}
