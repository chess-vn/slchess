package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/bucket-sort/slchess/pkg/logging"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Handler for saving current game state.
func (s *server) handleSaveGame(session *Session) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("ap-southeast-2"))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}
	lambdaClient := lambda.NewFromConfig(cfg)

	// Payload (optional)
	payload := []byte(`{"key": "value"}`)

	// Invoke Lambda function
	input := &lambda.InvokeInput{
		FunctionName:   aws.String(s.config.GameStatePutFunctionName),
		Payload:        payload,
		InvocationType: types.InvocationTypeEvent,
	}

	_, err = lambdaClient.Invoke(context.TODO(), input)
	if err != nil {
		logging.Error("failed to invoke end game", zap.Error(err))
	}

	s.removeSession(session.id)
	logging.Info("game ended", zap.String("session_id", session.id))
}

// Handler for when a game session ends.
func (s *server) handleEndGame(session *Session) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}
	lambdaClient := lambda.NewFromConfig(cfg)

	payload := map[string]interface{}{
		"matchId": session.id,
		"player1": session.players[0].Handler,
		"player2": session.players[1].Handler,
	}
	payloadJson, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Invoke Lambda function
	input := &lambda.InvokeInput{
		FunctionName:   aws.String(s.config.EndGameFunctionName),
		Payload:        payloadJson,
		InvocationType: types.InvocationTypeEvent,
	}

	_, err = lambdaClient.Invoke(context.TODO(), input)
	if err != nil {
		logging.Error("failed to invoke end game", zap.Error(err))
	}

	s.removeSession(session.id)
	logging.Info("game ended", zap.String("match_id", session.id))
}

// Handler for when a user connection closes
func (s *server) handlePlayerDisconnect(session *Session, playerHandler string) {
	if session == nil {
		return
	}

	player, exist := session.getPlayerWithHandler(playerHandler)
	if !exist {
		logging.Fatal("invalid player handler", zap.String("player_handler", playerHandler))
		return
	}
	player.Conn = nil
	player.Status = DISCONNECTED

	// If both player disconnected, end session
	if session.players[0].Status == session.players[1].Status {
		logging.Info("both player disconnected", zap.String("session_id", session.id))
		session.end()
	} else {
		// Else only set the timer for the disconnected player
		logging.Info("player disconnected", zap.String("session_id", session.id), zap.String("player_id", player.Handler))
		if !session.isEnded() {
			session.setTimer(60 * time.Second)
		}
	}
}

func (s *server) handlePlayerJoin(conn *websocket.Conn, session *Session, playerHandler string) {
	if session == nil {
		return
	}

	player, exist := session.getPlayerWithHandler(playerHandler)
	if !exist {
		logging.Fatal("invalid player handler", zap.String("player_handler", playerHandler))
		return
	}
	if player.Status == INIT && player.Side == WHITE_SIDE {
		session.startAt = time.Now()
		player.TurnStartedAt = session.startAt
		session.setTimer(session.config.MatchDuration)
	}
	player.Conn = conn
	player.Status = CONNECTED

	logging.Info("player connected",
		zap.String("player_handler", playerHandler),
		zap.String("session_id", session.id),
	)
}

// Handler for when user sends a message
func (s *server) handleWebSocketMessage(playerId string, session *Session, payload payload) {
	if session == nil {
		logging.Error("session not loaded")
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
			session.processGameControl(playerId, RESIGNAION)
		case "offer_draw":
			session.processGameControl(playerId, DRAW_OFFER)
		case "agreement":
			session.processGameControl(playerId, AGREEMENT)
		case "move":
			session.processMove(playerId, payload.Data["move"])
		default:
			logging.Info("invalid game action:", zap.String("action", payload.Type))
			return
		}
		logging.Info("game data",
			zap.String("sessionId", session.id),
			zap.String("action", action),
		)
	default:
		logging.Info("invalid payload type:", zap.String("type", payload.Type))
	}
}
