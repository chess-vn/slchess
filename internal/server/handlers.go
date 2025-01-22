package server

import (
	"github.com/bucket-sort/slchess/pkg/logging"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

/*
Handler for when a game instance ended.
This includes saving the session to the database, close the session
and remove session from tracking of Matcher
*/
func (s *Server) handleEndGame(session *Session) {
	// TODO: Call lambda EndGame
}

/*
Handler for when a user connection closes
*/
func (s *Server) handlePlayerDisconnect(session *Session) {
	if session == nil {
		return
	}
	playerID := "to_retrieve"
	sessionID := "to_retrieve"

	session.PlayerLeave(playerID)

	logging.Info("player disconnected",
		zap.String("player_id", playerID),
		zap.String("session_id", sessionID),
	)
}

/*
Handler for when user socket sends a message
*/
func (s *Server) handleWebSocketMessage(conn *websocket.Conn, session *Session, message *Message) {
	switch message.Type {
	case "move":
		playerId, playerOk := message.Data["player_id"].(string)
		sessionId, sessionOk := message.Data["session_id"].(string)
		move, moveOk := message.Data["move"].(string)
		if !playerOk || !sessionOk || !moveOk {
			logging.Info("attempt making move",
				zap.String("status", "rejected"),
				zap.String("error", "insufficient data"),
				zap.String("remote_address", conn.RemoteAddr().String()),
			)
			conn.WriteJSON(errorResponse{
				Type:  "error",
				Error: "insufficient data",
			})
			return
		}

		if session == nil {
			v, err := s.LoadSession(sessionId)
			if err != nil {
				conn.WriteJSON(errorResponse{
					Type:  "error",
					Error: "session not loaded",
				})
			}
			session = v
		}

		logging.Info("attempt making move",
			zap.String("status", "processing"),
			zap.String("player_id", playerId),
			zap.String("session_id", sessionId),
			zap.String("move", move),
			zap.String("remote_address", conn.RemoteAddr().String()),
		)
		session.ProcessMove(playerId, move)
	default:
	}
}
