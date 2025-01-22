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
	session.SetPlayer(player)

	// If both player disconnected, end session
	if session.Players[0].Status == session.Players[1].Status {
		session.End()
	} else {
		// Else only set the timer for the disconnected player
		session.Timer.Reset(60 * time.Second)
	}

	logging.Info("player disconnected",
		zap.String("player_id", playerId),
		zap.String("session_id", session.Id),
	)
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
		session.setTimer(session.Config.MatchDuration)
	}
	player.Conn = conn
	player.Status = CONNECTED
	session.SetPlayer(player)

	logging.Info("player connected",
		zap.String("player_id", playerId),
		zap.String("session_id", session.Id),
	)
}

// Handler for when user sends a message
func (s *Server) handleWebSocketMessage(playerId string, session *Session, payload *Payload) {
	if session == nil {
		logging.Error("session not loaded")
		return
	}
	switch payload.Type {
	case "chat":
		_ = payload.Data["message"]
	case "game_data":
		action := payload.Data["action"]
		switch action {
		case "resign":
			session.ProcessGameControl(playerId, RESIGNAION, payload.CreatedAt)
		case "offer_draw":
			session.ProcessGameControl(playerId, DRAW_OFFER, payload.CreatedAt)
		case "agreement":
			session.ProcessGameControl(playerId, AGREEMENT, payload.CreatedAt)
		case "move":
			session.ProcessMove(playerId, payload.Data["move"], payload.CreatedAt)
		default:
			logging.Info("invalid game action:", zap.String("action", payload.Type))
		}
	default:
		logging.Info("invalid payload type:", zap.String("type", payload.Type))
	}
}
