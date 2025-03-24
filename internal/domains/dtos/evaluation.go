package dtos

import (
	"github.com/chess-vn/slchess/internal/domains/entities"
)

type PvLichess struct {
	Cp    int    `json:"cp"`
	Moves string `json:"moves"`
}

type EvaluationLichess struct {
	Fen    string      `json:"fen"`
	Depth  int         `json:"depth"`
	Knodes int         `json:"knodes"`
	Pvs    []PvLichess `json:"pvs"`
}

type PvResponse struct {
	Cp    int    `json:"cp"`
	Moves string `json:"moves"`
}

type EvaluationResponse struct {
	Fen    string       `json:"fen"`
	Depth  int          `json:"depth"`
	Knodes int          `json:"knodes"`
	Pvs    []PvResponse `json:"pvs"`
}

func EvaluationLichessToEntity(eval EvaluationLichess) entities.Evaluation {
	v := entities.Evaluation{
		Fen:    eval.Fen,
		Depth:  eval.Depth,
		Knodes: eval.Knodes,
		Pvs:    make([]entities.Pv, 0, len(eval.Pvs)),
	}
	for _, pv := range eval.Pvs {
		v.Pvs = append(v.Pvs, entities.Pv{
			Cp:    pv.Cp,
			Moves: pv.Moves,
		})
	}
	return v
}

func EvaluationResponseFromEntity(eval entities.Evaluation) EvaluationResponse {
	v := EvaluationResponse{
		Fen:    eval.Fen,
		Depth:  eval.Depth,
		Knodes: eval.Knodes,
		Pvs:    make([]PvResponse, 0, len(eval.Pvs)),
	}
	for _, pv := range eval.Pvs {
		v.Pvs = append(v.Pvs, PvResponse{
			Cp:    pv.Cp,
			Moves: pv.Moves,
		})
	}
	return v
}
