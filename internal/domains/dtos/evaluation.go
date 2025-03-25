package dtos

import (
	"strings"

	"github.com/chess-vn/slchess/internal/domains/entities"
	"github.com/freeeve/uci"
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

type EvaluationRequest struct {
	ConnectionId string `json:"connectionId"`
	Fen          string `json:"fen"`
}

type EvaluationWorkResponse struct {
	ConnectionId  string `json:"connectionId"`
	Fen           string `json:"fen"`
	ReceiptHandle string `json:"receiptHandle"`
}

type EvaluationSubmission struct {
	ConnectionId  string       `json:"connectionId"`
	Fen           string       `json:"fen"`
	ReceiptHandle string       `json:"receiptHandle"`
	Results       *uci.Results `json:"results"`
}

func EvaluationWorkFromRequest(req EvaluationRequest) entities.EvaluationWork {
	return entities.EvaluationWork{
		ConnectionId: req.ConnectionId,
		Fen:          req.Fen,
	}
}

func EvaluationWorkResponseFromEntity(work entities.EvaluationWork) EvaluationWorkResponse {
	return EvaluationWorkResponse{
		ConnectionId:  work.ConnectionId,
		Fen:           work.Fen,
		ReceiptHandle: work.ReceiptHandle,
	}
}

func EvaluationSubmissionToEntity(submission EvaluationSubmission) entities.Evaluation {
	eval := entities.Evaluation{
		Fen:    submission.Fen,
		Depth:  submission.Results.Results[0].Depth,
		Knodes: submission.Results.Results[0].Nodes,
		Pvs:    make([]entities.Pv, len(submission.Results.Results)),
	}
	for _, result := range submission.Results.Results {
		pv := entities.Pv{
			Cp:    result.Score,
			Moves: strings.Join(result.BestMoves, " "),
		}
		eval.Pvs = append(eval.Pvs, pv)
	}
	return eval
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
