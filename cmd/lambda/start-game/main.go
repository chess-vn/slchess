package main

import (
	"github.com/bucket-sort/slchess/internal/handlers"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handlers.StartGameHandler)
}
