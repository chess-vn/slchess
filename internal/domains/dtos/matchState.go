package dtos

import (
	_ "embed"
	"time"

	"github.com/chess-vn/slchess/internal/domains/entities"
)

//go:embed graphql/updateMatchState.graphql
var updateMatchStateMutation string

type MatchStateRequest struct {
	MatchId   string               `json:"matchId"`
	Players   []PlayerStateRequest `json:"players"`
	GameState string               `json:"gameState"`
	UpdatedAt time.Time            `json:"updatedAt"`
}

type PlayerStateRequest struct {
	Clock  string `json:"clock"`
	Status string `json:"status"`
}

type MatchStateAppSyncRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

func NewMatchStateAppSyncRequest(req MatchStateRequest) MatchStateAppSyncRequest {
	return MatchStateAppSyncRequest{
		Query: updateMatchStateMutation,
		Variables: map[string]interface{}{
			"input": req,
		},
	}
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
