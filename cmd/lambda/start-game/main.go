package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/notnil/chess"
)

type GameState struct {
	GameID     string `json:"GameID"`
	Player1    string `json:"Player1"`
	Player2    string `json:"Player2"`
	State      string `json:"State"`
	NextTurn   string `json:"NextTurn"`
	GameStatus string `json:"GameStatus"`
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Parse request body
	var input struct {
		Player1 string `json:"Player1"`
		Player2 string `json:"Player2"`
	}
	err := json.Unmarshal([]byte(request.Body), &input)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400}, fmt.Errorf("invalid request body: %v", err)
	}

	// Generate unique GameID
	gameID := fmt.Sprintf("game-%d", time.Now().Unix())

	// Initialize game state
	game := chess.NewGame()

	gameState := GameState{
		GameID:     gameID,
		Player1:    input.Player1,
		Player2:    input.Player2,
		State:      game.FEN(), // Replace with actual board initialization logic
		NextTurn:   input.Player1,
		GameStatus: "ongoing",
	}

	// Save game state to DynamoDB
	sess := session.Must(session.NewSession())
	svc := dynamodb.New(sess)

	item, err := dynamodbattribute.MarshalMap(gameState)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500}, fmt.Errorf("failed to marshal game: %v", err)
	}

	_, err = svc.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String("GameStates"),
		Item:      item,
	})
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500}, fmt.Errorf("failed to save game: %v", err)
	}

	// Return success response
	response := map[string]string{
		"Message": "Game started successfully",
		"GameID":  gameID,
	}
	responseBody, _ := json.Marshal(response)

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(responseBody),
	}, nil
}

func main() {
	lambda.Start(handler)
}
