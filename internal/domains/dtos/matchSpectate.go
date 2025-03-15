package dtos

import (
	"reflect"

	"github.com/chess-vn/slchess/internal/domains/entities"
)

type MatchSpectateResponse struct {
	MatchState     *MatchStateResponse `json:"matchState"`
	ConversationId string              `json:"conversationId"`
}

func NewMatchSpectateResponse(matchState entities.MatchState, conversationId string) MatchSpectateResponse {
	var matchStateResp *MatchStateResponse
	if !reflect.DeepEqual(matchState, entities.MatchState{}) {
		resp := MatchStateResponseFromEntitiy(matchState)
		matchStateResp = &resp
	}
	return MatchSpectateResponse{
		MatchState:     matchStateResp,
		ConversationId: conversationId,
	}
}
