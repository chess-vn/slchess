package server

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/bucket-sort/slchess/pkg/logging"
	"github.com/gorilla/websocket"
	"github.com/notnil/chess"
	"go.uber.org/zap"
)

type Server struct {
	address  string
	upgrader websocket.Upgrader

	sessions sync.Map
}

type Message struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
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
		var session *Session
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
				s.handlePlayerDisconnect(session)
				break
			}

			msg := Message{}
			if err := json.Unmarshal(message, &msg); err != nil {
				conn.Close()
			}
			s.handleWebSocketMessage(conn, session, &msg)
		}
	})
	logging.Info("websocket server started", zap.String("port", Port))
	return http.ListenAndServe(s.address, nil)
}

// LoadSession method    loads session with corresponding sessionId.
// If no such session exists, create a new session.
// This is used to start the match only when white side player send in the first valid move.
func (s *Server) LoadSession(sessionId string) (*Session, error) {
	// TODO: fetch session info from dynamoDB
	// to validate sessionId and create new session if needed
	value, loaded := s.sessions.LoadOrStore(sessionId, &Session{})
	if loaded {
		session, ok := value.(*Session)
		if ok {
			return session, nil
		}
	}
	return nil, ErrLoadSessionFailure
}

func (s *Server) NewSession(sessionId string, player1, player2 *Player) {
	session := &Session{
		Game: chess.NewGame(chess.UseNotation(chess.UCINotation{})),
		Players: map[string]*Player{
			player1.Id: player1,
			player2.Id: player2,
		},
		MoveCh: make(chan move),
	}
	go session.Start()
	s.sessions.Store(sessionId, session)
}

func (s *Server) removeSession(sessionId string) {
	s.sessions.Delete(sessionId)
}
