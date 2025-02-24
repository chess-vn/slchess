package dtos

import (
	"time"

	"github.com/chess-vn/slchess/internal/domains/entities"
)

type MatchStateRequest struct {
	MatchId   string               `json:"matchId"`
	Players   []PlayerStateRequest `json:"players"`
	GameState string               `json:"gameState"`
	UpdatedAt time.Time            `json:"updatedAt"`
}

type PlayerStateRequest struct {
	Clock  string `dynamodbav:"Clock" json:"clock"`
	Status string `dynamodbav:"Status" json:"status"`
}

func MatchStateRequestToEntity(req MatchStateRequest) entities.MatchState {
	return entities.MatchState{
		MatchId: req.MatchId,
		Players: []entities.PlayerState{
			{
				Clock:  req.Players[0].Clock,
				Status: req.Players[0].Status,
			},
			{
				Clock:  req.Players[1].Clock,
				Status: req.Players[1].Status,
			},
		},
		GameState: req.GameState,
		UpdatedAt: req.UpdatedAt,
	}
}
