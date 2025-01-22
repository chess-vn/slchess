package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/bucket-sort/slchess/pkg/logging"
	"github.com/gorilla/websocket"
	"github.com/notnil/chess"
	"go.uber.org/zap"
)

type Server struct {
	address  string
	upgrader websocket.Upgrader

	config   Config
	sessions sync.Map
}

type Payload struct {
	Type      string
	Data      map[string]string
	CreatedAt time.Time
}

func NewServer() *Server {
	config := NewConfig()
	return &Server{
		address: "0.0.0.0:" + config.Port,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins
			},
		},
		config: config,
	}
}

// Start method    starts the game server
func (s *Server) Start() error {
	http.HandleFunc("/sessions/{sessionId}", func(w http.ResponseWriter, r *http.Request) {
		playerId := s.MustAuth(r)

		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			logging.Error("failed to upgrade connection", zap.String("error", err.Error()))
			return
		}
		defer conn.Close()

		sessionId := r.PathValue("sessionId")
		session, err := s.LoadSession(sessionId)
		if err != nil {
			logging.Error("failed to load session", zap.String("error", err.Error()))
			return
		}
		s.handlePlayerJoin(conn, session, playerId)

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
				s.handlePlayerDisconnect(session, playerId)
				break
			}

			payload := Payload{}
			if err := json.Unmarshal(message, &payload); err != nil {
				conn.Close()
			}
			s.handleWebSocketMessage(playerId, session, &payload)
		}
	})
	logging.Info("websocket server started", zap.String("port", s.config.Port))
	return http.ListenAndServe(s.address, nil)
}

// MustAuth method    authenticates and extract playerId
func (s *Server) MustAuth(r *http.Request) string {
	_ = r.Header.Get("Authorization")
	return "player-id"
}

// LoadSession method    loads session with corresponding sessionId.
// If no such session exists, create a new session.
// This is used to start the match only when white side player send in the first valid move.
func (s *Server) LoadSession(sessionId string) (*Session, error) {
	// TODO: fetch session info from dynamoDB
	// to validate sessionId and create new session if needed
	config := SessionConfig{
		MatchDuration: 10 * time.Minute,
	}
	player1 := NewPlayer(nil, "PLAYER_1", WHITE_SIDE, config.MatchDuration)
	player2 := NewPlayer(nil, "PLAYER_2", BLACK_SIDE, config.MatchDuration)

	value, loaded := s.sessions.LoadOrStore(
		sessionId,
		s.NewSession(sessionId, player1, player2, config),
	)
	if loaded {
		session, ok := value.(*Session)
		if ok {
			return session, nil
		}
	}
	return nil, ErrLoadSessionFailure
}

func (s *Server) NewSession(sessionId string, player1, player2 Player, config SessionConfig) *Session {
	session := &Session{
		Id:              sessionId,
		Game:            chess.NewGame(chess.UseNotation(chess.UCINotation{})),
		Players:         []Player{player1, player2},
		moveCh:          make(chan Move),
		Config:          config,
		EndGameHandler:  s.handleEndGame,
		SaveGameHandler: s.handleSaveGame,
	}
	go session.Start()
	return session
}

func (s *Server) removeSession(sessionId string) {
	s.sessions.Delete(sessionId)
}
