package stofinet

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/chess-vn/slchess/internal/domains/dtos"
	"github.com/chess-vn/slchess/pkg/logging"
	"go.uber.org/zap"
)

func fenAnalyseHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logging.Error("failed to read body", zap.Error(err))
	}

	var req dtos.EvaluationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logging.Error("faield to unmarshal request: %w", zap.Error(err))
	}
}
