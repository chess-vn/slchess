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
	engine, err := uci.NewEngine("/usr/bin/stockfish")
	if err != nil {
		logging.Fatal("couldn't initialize engine", zap.Error(err))
	}
	engine.SetOptions(uci.Options{
		Threads: cfg.NumThreads,
		MultiPV: 3,
		Hash:    cfg.HashSize,
		Ponder:  false,
	})
	return &Client{
		engine: engine,
		http:   new(http.Client),
		cfg:    cfg,
	}
}

func (client *Client) Start(ctx context.Context) error {
	backoffTime := 2 * time.Second
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		work, err := client.AcquireEvaluationWork(ctx)
		if err != nil {
			if errors.Is(err, ErrEvaluationWorkNotFound) {
				time.Sleep(backoffTime)
				continue
			}
			return fmt.Errorf("failed to acquire evaluation work: %w", err)
		}

		results, err := client.Evaluate(work.Fen, 25)
		if err != nil {
			return fmt.Errorf("failed to evaluate: %w", err)
		}
		logging.Info("done evaluating")

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
		logging.Info("evaluation submitted")
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
	logging.Info("acquired work", zap.String("status", resp.Status))

	switch resp.StatusCode {
	case http.StatusOK:
		var eval dtos.EvaluationWorkResponse
		if err := json.NewDecoder(resp.Body).Decode(&eval); err != nil {
			return dtos.EvaluationWorkResponse{}, fmt.Errorf("failed to decode body: %w", err)
		}
		return eval, nil
	case http.StatusNotFound:
		return dtos.EvaluationWorkResponse{}, ErrEvaluationWorkNotFound
	default:
		return dtos.EvaluationWorkResponse{}, ErrUnknownStatusCode
	}
}

func (client *Client) SubmitEvaluation(ctx context.Context, sub dtos.EvaluationSubmission) error {
	// subJson, err:= json.Marshal(sub)
	u := client.cfg.BaseUrl.JoinPath("evaluation")

	bodyJson, err := json.Marshal(sub)
	if err != nil {
		return fmt.Errorf(" failed to marshal body: %w", err)
	}
	body := bytes.NewReader(bodyJson)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.http.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	logging.Info(
		"evaluation submission",
		zap.Int("status_code", resp.StatusCode),
	)

	return nil
}

func (client *Client) Evaluate(fen string, depth int) (*uci.Results, error) {
	logging.Info("evaluating", zap.String("fen", fen))
	if err := client.engine.SetFEN(fen); err != nil {
		return nil, fmt.Errorf("failed to set fen: %w", err)
	}
	resultOpts := uci.HighestDepthOnly | uci.IncludeLowerbounds | uci.IncludeUpperbounds
	results, err := client.engine.GoDepth(depth, resultOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to go depth %d: %w", depth, err)
	}
	return results, nil
}
