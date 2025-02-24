package server

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/chess-vn/slchess/internal/domains/dtos"
	"github.com/chess-vn/slchess/pkg/logging"
	"github.com/gorilla/websocket"
	"github.com/notnil/chess"
	"go.uber.org/zap"
)

// Handler for saving current game state.
func (s *server) handleSaveGame(match *Match) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}
	lambdaClient := lambda.NewFromConfig(cfg)

	matchStateReq := dtos.MatchStateRequest{
		MatchId: match.id,
		Players: []dtos.PlayerStateRequest{
			{
				Clock:  match.players[0].Clock.String(),
				Status: match.players[0].Status.String(),
			},
			{
				Clock:  match.players[1].Clock.String(),
				Status: match.players[1].Status.String(),
			},
		},
		GameState: match.game.FEN(),
		UpdatedAt: time.Now(),
	}
	payload, err := json.Marshal(matchStateReq)
	if err != nil {
		log.Fatal(err)
	}

	// Invoke Lambda function
	input := &lambda.InvokeInput{
		FunctionName:   aws.String(s.config.GameStatePutFunctionName),
		Payload:        payload,
		InvocationType: types.InvocationTypeEvent,
	}

	_, err = lambdaClient.Invoke(context.TODO(), input)
	if err != nil {
		logging.Error("failed to invoke save game", zap.Error(err))
	}
}

// Handler for when a game match ends.
func (s *server) handleEndGame(match *Match) {
	if match == nil {
		return
	}
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		logging.Fatal("unable to load SDK config", zap.Error(err))
	}
	lambdaClient := lambda.NewFromConfig(cfg)

	newRatings, newRDs, err := match.getNewPlayerRatings()
	if err != nil {
		logging.Fatal("failed to invoke end game", zap.Error(err))
	}
	matchRecordReq := dtos.MatchRecordRequest{
		MatchId: match.id,
		Players: []dtos.PlayerRecordRequest{
			{
				Id:        match.players[0].Id,
				OldRating: match.players[0].Rating,
				NewRating: newRatings[0],
				OldRD:     match.players[0].RD,
				NewRD:     newRDs[0],
			},
			{
				Id:        match.players[1].Id,
				OldRating: match.players[1].Rating,
				NewRating: newRatings[1],
				NewRD:     newRDs[1],
				OldRD:     match.players[1].RD,
			},
		},
		Pgn:       match.game.String(),
		StartedAt: match.startAt,
		EndedAt:   time.Now(),
	}
	switch match.game.outcome() {
	case chess.WhiteWon:
		matchRecordReq.Results = []float64{1.0, 0.0}
	case chess.BlackWon:
		matchRecordReq.Results = []float64{0.0, 1.0}
	case chess.Draw, chess.NoOutcome:
		matchRecordReq.Results = []float64{0.5, 0.5}
	}
	payload, err := json.Marshal(matchRecordReq)
	if err != nil {
		log.Fatal(err)
	}

	// Invoke Lambda function
	input := &lambda.InvokeInput{
		FunctionName:   aws.String(s.config.EndGameFunctionName),
		Payload:        payload,
		InvocationType: types.InvocationTypeEvent,
	}

	_, err = lambdaClient.Invoke(context.TODO(), input)
	if err != nil {
		logging.Fatal("failed to invoke end game", zap.Error(err))
	}

	s.removeMatch(match.id)
	logging.Info("match ended", zap.String("match_id", match.id))
}

// Handler for when a user connection closes
func (s *server) handlePlayerDisconnect(match *Match, playerId string) {
	if match == nil {
		return
	}

	player, exist := match.getPlayerWithId(playerId)
	if !exist {
		logging.Fatal("invalid player id", zap.String("player_id", playerId))
		return
	}
	player.Conn = nil
	player.Status = DISCONNECTED

	// If both player disconnected, end match
	if match.players[0].Status == match.players[1].Status {
		logging.Info("both player disconnected", zap.String("match_id", match.id))
		match.end()
	} else {
		// Else only set the timer for the disconnected player
		logging.Info("player disconnected", zap.String("match_id", match.id), zap.String("player_id", player.Id))
		if !match.isEnded() {
			match.setTimer(60 * time.Second)
		}
	}
}

func (s *server) handlePlayerJoin(conn *websocket.Conn, match *Match, playerId string) {
	if match == nil {
		return
	}

	player, exist := match.getPlayerWithId(playerId)
	if !exist {
		logging.Fatal("invalid player id", zap.String("player_id", playerId))
		return
	}
	if player.Status == INIT && player.Side == WHITE_SIDE {
		match.startAt = time.Now()
		player.TurnStartedAt = match.startAt
		match.setTimer(match.config.MatchDuration)
	}
	player.Conn = conn
	player.Status = CONNECTED

	match.syncPlayer(player)

	logging.Info("player connected",
		zap.String("player_id", playerId),
		zap.String("match_id", match.id),
	)
}

// Handler for when user sends a message
func (s *server) handleWebSocketMessage(playerId string, match *Match, payload payload) {
	if match == nil {
		logging.Error("match not loaded")
		return
	}
	// Validate timestamp
	if time.Since(payload.CreatedAt) < 0 {
		logging.Info("invalid timestamp",
			zap.String("created_at", payload.CreatedAt.String()),
			zap.String("validate_time", time.Now().String()),
		)
		return
	}
	switch payload.Type {
	case "chat":
		_ = payload.Data["message"]
	case "game_data":
		action := payload.Data["action"]
		switch action {
		case "resign":
			match.processGameControl(playerId, RESIGNAION)
		case "offer_draw":
			match.processGameControl(playerId, DRAW_OFFER)
		case "agreement":
			match.processGameControl(playerId, AGREEMENT)
		case "move":
			match.processMove(playerId, payload.Data["move"])
		default:
			logging.Info("invalid game action:", zap.String("action", payload.Type))
			return
		}
		logging.Info("game data",
			zap.String("match_id", match.id),
			zap.String("action", action),
		)
	case "sync":
		match.syncPlayerWithId(playerId)
	default:
		logging.Info("invalid payload type:", zap.String("type", payload.Type))
	}
}
