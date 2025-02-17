package entities

import "time"

type MatchmakingTicket struct {
	UserHandler string  `dynamodbav:"UserHandler"`
	Rating      float64 `dynamodbav:"Rating"`
	MinRating   float64 `dynamodbav:"MinRating"`
	MaxRating   float64 `dynamodbav:"MaxRating"`
	RD          float64 `dynamodbav:"RD"`
	GameMode    string  `dynamodbav:"GameMode"`
}

type ActiveMatch struct {
	MatchId   string    `dynamodbav:"MatchId"`
	Player1   Player    `dynamodbav:"Player1"`
	Player2   Player    `dynamodbav:"Player2"`
	GameMode  string    `dynamodbav:"GameMode"`
	Server    string    `dynamodbav:"GameMode"`
	CreatedAt time.Time `dynamodbav:"CreatedAt"`
}

type Player struct {
	Handler       string    `dynamodbav:"Handler"`
	Rating        float64   `dynamodbav:"Rating"`
	RD            float64   `dynamodbav:"RD"`
	RatingChanges []float64 `dynamodbav:"RatingChanges"`
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

type PlayerRecord struct {
	Handler   string  `dynamodbav:"Id" json:"id"`
	OldRating float64 `dynamodbav:"Rating" json:"rating"`
	NewRating float64 `dynamodbav:"NewRating" json:"newRating"`
}

type MatchRecord struct {
	MatchId   string         `dynamodbav:"MatchId" json:"matchId"`
	Players   []PlayerRecord `dynamodbav:"Players" json:"players"`
	Pgn       string         `dynamodbav:"Pgn" json:"pgn"`
	StartedAt time.Time      `dynamodbav:"StartedAt" json:"startedAt"`
	EndedAt   time.Time      `dynamodbav:"EndedAt" json:"endedAt"`
}
