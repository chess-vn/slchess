package main

import (
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"
)

func handler() {
	fmt.Println("Game saved")
}

func main() {
	lambda.Start(handler)
}
