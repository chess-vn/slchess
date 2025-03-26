package stofinet

import "github.com/chess-vn/slchess/internal/domains/dtos"

func EvaluationResultFromStofinet(eval Evaluation) dtos.EvaluationResult {
	v := dtos.EvaluationResult{
		Fen:    eval.Fen,
		Depth:  eval.Depth,
		Knodes: eval.Knodes,
		Pvs:    make([]dtos.PvResult, 0, len(eval.Pvs)),
	}
	for _, pv := range eval.Pvs {
		v.Pvs = append(v.Pvs, dtos.PvResult{
			Cp:    pv.Cp,
			Moves: pv.Moves,
		})
	}
	return v
}
