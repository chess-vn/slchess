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
	Clock      string    `dynamodbav:"clock" json:"clock"`
	LastMoveAt time.Time `dynamodbav:"lastMoveAt" json:"lastMoveAt"`
	Status     string    `dynamodbav:"status" json:"status"`
}

type MatchState struct {
	MatchId   string        `dynamodbav:"matchId" json:"matchId"`
	Players   []PlayerState `dynamodbav:"players" json:"players"`
	GameState string        `dynamodbav:"gameState" json:"gameState"`
	UpdatedAt time.Time     `dynamodbav:"updatedAt" json:"updatedAt"`
}
