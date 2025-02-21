package entities

import (
	"time"
)

type MatchmakingTicket struct {
	UserId    string  `dynamodbav:"UserId"`
	MinRating float64 `dynamodbav:"MinRating" json:"minRating"`
	MaxRating float64 `dynamodbav:"MaxRating" json:"maxRating"`
	GameMode  string  `dynamodbav:"GameMode" json:"gameMode"`
}

type ActiveMatch struct {
	MatchId   string    `dynamodbav:"MatchId" json:"MatchId"`
	Player1   Player    `dynamodbav:"Player1" json:"Player1"`
	Player2   Player    `dynamodbav:"Player2" json:"Player2"`
	GameMode  string    `dynamodbav:"GameMode" json:"GameMode"`
	Server    string    `dynamodbav:"GameMode" json:"Server"`
	CreatedAt time.Time `dynamodbav:"CreatedAt" json:"CreatedAt"`
}

type Player struct {
	Id         string    `dynamodbav:"Id" json:"Id"`
	Rating     float64   `dynamodbav:"Rating" json:"Rating"`
	RD         float64   `dynamodbav:"RD" json:"-"`
	NewRatings []float64 `dynamodbav:"NewRatings" json:"NewRatings"`
	NewRDs     []float64 `dynamodbav:"NewRDs" json:"-"`
}

type UserMatch struct {
	UserId  string `dynamodbav:"UserId"`
	MatchId string `dynamodbav:"MatchId"`
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
	Id        string  `dynamodbav:"Id" json:"id"`
	OldRating float64 `dynamodbav:"Rating" json:"rating"`
	NewRating float64 `dynamodbav:"NewRating" json:"newRating"`
	OldRD     float64 `json:"oldRD"`
	NewRD     float64 `json:"newRD"`
}

type MatchRecord struct {
	MatchId   string         `dynamodbav:"MatchId" json:"matchId"`
	Players   []PlayerRecord `dynamodbav:"Players" json:"players"`
	Pgn       string         `dynamodbav:"Pgn" json:"pgn"`
	StartedAt time.Time      `dynamodbav:"StartedAt" json:"startedAt"`
	EndedAt   time.Time      `dynamodbav:"EndedAt" json:"endedAt"`
	Results   []float64      `json:"results"`
}

type MatchResult struct {
	UserId         string  `dynamodbav:"UserId"`
	MatchId        string  `dynamodbav:"MatchId"`
	OpponentId     string  `dynamodbav:"OpponentId"`
	OpponentRating float64 `dynamodbav:"OpponentRating"`
	OpponentRD     float64 `dynamodbav:"OpponentRD"`
	Result         float64 `dynamodbav:"Result"`
	Timestamp      string  `dynamodbav:"Timestamp"`
}
