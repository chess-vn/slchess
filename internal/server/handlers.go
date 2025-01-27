package server

import (
	"time"

	"github.com/bucket-sort/slchess/pkg/logging"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Handler for saving current game state.
func (s *Server) handleSaveGame(session *Session) {
	// TODO: Call lambda GameStatePut
}

// Handler for when a game session ends.
func (s *Server) handleEndGame(session *Session) {
	// TODO: Call lambda EndGame
	s.RemoveSession(session.Id)
	logging.Info("game ended", zap.String("session_id", session.Id))
}

// Handler for when a user connection closes
func (s *Server) handlePlayerDisconnect(session *Session, playerId string) {
	if session == nil {
		return
	}

	player, exist := session.GetPlayerWithId(playerId)
	if !exist {
		logging.Fatal("invalid player id", zap.String("player_id", playerId))
		return
	}
	player.Conn = nil
	player.Status = DISCONNECTED

	// If both player disconnected, end session
	if session.Players[0].Status == session.Players[1].Status {
		logging.Info("both player disconnected", zap.String("session_id", session.Id))
		session.End()
	} else {
		// Else only set the timer for the disconnected player
		logging.Info("player disconnected", zap.String("session_id", session.Id), zap.String("player_id", player.Id))
		if !session.IsEnded() {
			session.SetTimer(60 * time.Second)
		}
	}
}

func (s *Server) handlePlayerJoin(conn *websocket.Conn, session *Session, playerId string) {
	if session == nil {
		return
	}

	player, exist := session.GetPlayerWithId(playerId)
	if !exist {
		logging.Fatal("invalid player id", zap.String("player_id", playerId))
		return
	}
	if player.Status == INIT && player.Side == WHITE_SIDE {
		session.StartAt = time.Now()
		player.TurnStartedAt = session.StartAt
		session.SetTimer(session.Config.MatchDuration)
	}
	player.Conn = conn
	player.Status = CONNECTED

	logging.Info("player connected",
		zap.String("player_id", playerId),
		zap.String("session_id", session.Id),
	)
}

// Handler for when user sends a message
func (s *Server) handleWebSocketMessage(playerId string, session *Session, payload Payload) {
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
			session.ProcessGameControl(playerId, RESIGNAION)
		case "offer_draw":
			session.ProcessGameControl(playerId, DRAW_OFFER)
		case "agreement":
			session.ProcessGameControl(playerId, AGREEMENT)
		case "move":
			session.ProcessMove(playerId, payload.Data["move"])
		default:
			logging.Info("invalid game action:", zap.String("action", payload.Type))
			return
		}
		logging.Info("game data",
			zap.String("sessionId", session.Id),
			zap.String("action", action),
		)
	default:
		logging.Info("invalid payload type:", zap.String("type", payload.Type))
	}
}
