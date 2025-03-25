package stofinet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/chess-vn/slchess/internal/domains/dtos"
	"github.com/chess-vn/slchess/pkg/logging"
	"github.com/freeeve/uci"
	"go.uber.org/zap"
)

type Client struct {
	engine *uci.Engine
	http   *http.Client
	cfg    Config
}

func NewClient() *Client {
	cfg, err := LoadConfig()
	if err != nil {
		logging.Fatal("couldn't load config", zap.Error(err))
	}
	engine, err := uci.NewEngine(cfg.StockfishPath)
	if err != nil {
		logging.Fatal("couldn't initialize engine", zap.Error(err))
	}
	engine.SetOptions(uci.Options{
		MultiPV: 3,
		Hash:    128,
		Ponder:  false,
		OwnBook: true,
	})
	return &Client{
		engine: engine,
		http:   new(http.Client),
		cfg:    cfg,
	}
}

func (client *Client) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		work, err := client.AcquireEvaluationWork(ctx)
		if err != nil {
			if errors.Is(err, ErrEvaluationWorkNotFound) {
				time.Sleep(1 * time.Second)
				continue
			}
			return fmt.Errorf("failed to acquire evaluation work")
		}

		results, err := client.Evaluate(work.Fen, 30)
		if err != nil {
			return fmt.Errorf("failed to evaluate: %w", err)
		}

		sub := dtos.EvaluationSubmission{
			ConnectionId:  work.ConnectionId,
			Fen:           work.Fen,
			ReceiptHandle: work.ReceiptHandle,
			Results:       results,
		}
		if err := client.SubmitEvaluation(ctx, sub); err != nil {
			logging.Error(
				"failed to submit evaluation: %w",
				zap.Error(err),
			)
		}

	}
}

func (client *Client) AcquireEvaluationWork(ctx context.Context) (dtos.EvaluationWorkResponse, error) {
	u := client.cfg.BaseUrl.JoinPath("acquire")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return dtos.EvaluationWorkResponse{}, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.http.Do(req)
	if err != nil {
		return dtos.EvaluationWorkResponse{}, fmt.Errorf("failed to send request: %w", err)
	}

	var eval dtos.EvaluationWorkResponse
	if err := json.NewDecoder(resp.Body).Decode(&eval); err != nil {
		return dtos.EvaluationWorkResponse{}, fmt.Errorf("failed to decode body: %w", err)
	}

	return eval, nil
}

func (client *Client) SubmitEvaluation(ctx context.Context, sub dtos.EvaluationSubmission) error {
	// subJson, err:= json.Marshal(sub)
	u := client.cfg.BaseUrl.JoinPath("evaluation")

	body := new(bytes.Buffer)
	if err := json.NewEncoder(body).Encode(sub); err != nil {
		return fmt.Errorf(" failed to encode body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.http.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	logging.Info(
		"evaluation summission",
		zap.Int("status_code", resp.StatusCode),
	)

	return nil
}
