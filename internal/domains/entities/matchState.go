package entities

import "time"

type PlayerState struct {
	Clock  string `dynamodbav:"Clock"`
	Status string `dynamodbav:"Status"`
}

type MatchState struct {
	MatchId   string        `dynamodbav:"MatchId"`
	Players   []PlayerState `dynamodbav:"Players"`
	GameState string        `dynamodbav:"GameState"`
	UpdatedAt time.Time     `dynamodbav:"UpdatedAt"`
}
