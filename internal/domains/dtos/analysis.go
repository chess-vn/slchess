package dtos

import (
	"github.com/chess-vn/slchess/internal/domains/entities"
	"github.com/freeeve/uci"
)

type FenAnalyseRequest struct {
	Id  string `json:"id"`
	Fen string `json:"fen"`
}

type FenAnalysisWorkResponse struct {
	Id  string `json:"id"`
	Fen string `json:"fen"`
}

type FenAnalysisSubmission struct {
	Results uci.Results `json:"results"`
}

func FenAnalysisRequestToEntity(req FenAnalyseRequest) entities.FenAnalysisWork {
	return entities.FenAnalysisWork{
		Id:  req.Id,
		Fen: req.Fen,
	}
}

func FenAnalysisWorkResponseFromEntity(work entities.FenAnalysisWork) FenAnalysisWorkResponse {
	return FenAnalysisWorkResponse{
		Id:  work.Id,
		Fen: work.Fen,
	}
}
