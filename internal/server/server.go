package server

import (
	"encoding/json"
	"net/http"

	"github.com/bucket-sort/slchess/pkg/logging"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type Server struct {
	address  string
	upgrader websocket.Upgrader
}

type Message struct {
	Action string                 `json:"action"`
	Data   map[string]interface{} `json:"data"`
}

func NewServer() *Server {
	return &Server{
		address: "0.0.0.0:" + Port,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins
			},
		},
	}
}

/*
Start the websocket server
*/
func (s *Server) Start() error {
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			logging.Error("failed to upgrade connection", zap.String("error", err.Error()))
			return
		}
		defer conn.Close()
		var connID string
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseMessage, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logging.Info("unexpected close error", zap.String("remote_address", conn.RemoteAddr().String()))
				} else if websocket.IsCloseError(err, websocket.CloseMessage, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logging.Info("connection closed", zap.String("remote_address", conn.RemoteAddr().String()))
				} else {
					logging.Info("ws message read error", zap.String("remote_address", conn.RemoteAddr().String()))
				}
				handlePlayerDisconnect(connID)
				break
			}

			msg := Message{}
			if err := json.Unmarshal(message, &msg); err != nil {
				conn.Close()
			}
			handleWebSocketMessage(conn, &msg, &connID)
		}
	})
	logging.Info("websocket server started", zap.String("port", Port))
	return http.ListenAndServe(s.address, nil)
}

/*
Handler for when a game instance ended.
This includes saving the session to the database, close the session
and remove session from tracking of Matcher
*/
func handleEndGame(s *GameSession, sessionID string) {
	// TODO: Call lambda EndGame

	CloseSession(sessionID)
}

/*
Handler for when a user connection closes
*/
func handlePlayerDisconnect(connID string) {
	playerID := "to_retrieve"
	sessionID := "to_retrieve"

	err := PlayerLeave(sessionID, playerID)
	if err != nil {
		logging.Warn("player disconnected error",
			zap.String("player_id", playerID),
			zap.String("session_id", sessionID),
			zap.Error(err),
		)
	}

	logging.Info("player disconnected",
		zap.String("player_id", playerID),
		zap.String("session_id", sessionID),
	)
}

/*
Handler for when user socket sends a message
*/
func handleWebSocketMessage(conn *websocket.Conn, message *Message, connID *string) {
	type errorResponse struct {
		Type  string `json:"type"`
		Error string `json:"error"`
	}
	switch message.Action {
	case "move":
		playerID, playerOK := message.Data["player_id"].(string)
		sessionID, sessionOK := message.Data["session_id"].(string)
		move, moveOK := message.Data["move"].(string)
		if playerOK && sessionOK && moveOK {
			logging.Info("attempt making move",
				zap.String("status", "processing"),
				zap.String("player_id", playerID),
				zap.String("session_id", sessionID),
				zap.String("move", move),
				zap.String("remote_address", conn.RemoteAddr().String()),
			)
			ProcessFenMove(sessionID, playerID, move)
		} else {
			logging.Info("attempt making move",
				zap.String("status", "rejected"),
				zap.String("error", "insufficient data"),
				zap.String("remote_address", conn.RemoteAddr().String()),
			)
			conn.WriteJSON(errorResponse{
				Type:  "error",
				Error: "insufficient data",
			})
		}
	default:
	}
}
