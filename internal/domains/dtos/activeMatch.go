package dtos

import (
	"time"

	"github.com/chess-vn/slchess/internal/domains/entities"
)

type ActiveMatchResponse struct {
	MatchId   string         `json:"MatchId"`
	Player1   PlayerResponse `json:"Player1"`
	Player2   PlayerResponse `json:"Player2"`
	GameMode  string         `json:"GameMode"`
	Server    string         `json:"Server"`
	CreatedAt time.Time      `json:"CreatedAt"`
}

type PlayerResponse struct {
	Id         string    `json:"Id"`
	Rating     float64   `json:"Rating"`
	NewRatings []float64 `json:"NewRatings"`
}

func ActiveMatchResponseFromEntity(activeMatch entities.ActiveMatch) ActiveMatchResponse {
	return ActiveMatchResponse{
		MatchId: activeMatch.MatchId,
		Player1: PlayerResponse{
			Id:         activeMatch.Player1.Id,
			Rating:     activeMatch.Player1.Rating,
			NewRatings: activeMatch.Player1.NewRatings,
		},
		Player2: PlayerResponse{
			Id:         activeMatch.Player2.Id,
			Rating:     activeMatch.Player2.Rating,
			NewRatings: activeMatch.Player2.NewRatings,
		},
		GameMode:  activeMatch.GameMode,
		Server:    activeMatch.Server,
		CreatedAt: activeMatch.CreatedAt,
	}
}
