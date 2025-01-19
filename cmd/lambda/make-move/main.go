package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/bucket-sort/slchess/internal/handlers"
)

func main() {
	lambda.Start(handlers.MakeMoveHandler)
}
