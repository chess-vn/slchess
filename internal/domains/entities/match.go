package entities

import "time"

type Ticket struct {
	UserId    string
	MinRating int
	MaxRating int
}

type Match struct {
	Id        string
	Player1   string
	Player2   string
	GameMode  string
	Server    string
	CreatedAt time.Time
}

type PlayerState struct {
	Clock  string `dynamodbav:"Clock" json:"clock"`
	Status string `dynamodbav:"Status" json:"status"`
}

type MatchState struct {
	MatchId   string        `dynamodbav:"MatchId" json:"matchId"`
	Players   []PlayerState `dynamodbav:"Players" json:"players"`
	GameState string        `dynamodbav:"GameState" json:"gameState"`
	UpdatedAt time.Time     `dynamodbav:"UpdatedAt" json:"updatedAt"`
}
