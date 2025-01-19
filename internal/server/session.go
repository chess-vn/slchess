package server

import (
	"errors"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/notnil/chess"
)

type GameSession struct {
	Players map[string]*Player
	Game    *chess.Game
}

type GameState struct {
	Status      string       `json:"status"`
	Board       [8][8]string `json:"board"`
	IsWhiteTurn bool         `json:"is_white"`
}

type SessionResponse struct {
	Type      string    `json:"type"`
	GameState GameState `json:"game_state"`
}

type Player struct {
	Conn *websocket.Conn
	ID   string `json:"id"`
}

type PlayerState struct {
	IsWhiteSide bool `json:"is_white_side"`
}

var (
	gameSessions    = make(map[string]*GameSession)
	mu              sync.RWMutex
	gameOverHandler = func(session *GameSession, sessionID string) {
		CloseSession(sessionID)
		for _, player := range session.Players {
			player.Conn.Close()
		}
	}
)

func InitSession(sessionID string, player1, player2 *Player) {
	playersMap := map[string]*Player{
		player1.ID: player1,
		player2.ID: player2,
	}
	gameSessions[sessionID] = &GameSession{
		Players: playersMap,
		Game:    chess.NewGame(),
	}
}

func CloseSession(sessionID string) {
	mu.Lock()
	defer mu.Unlock()
	delete(gameSessions, sessionID)
}

func SetGameOverHandler(govHandler func(*GameSession, string)) {
	gameOverHandler = govHandler
}

func StartGame(session *GameSession) {
	for _, player := range session.Players {
		err := player.Conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"start"}`))
		if err != nil {
			log.Println("Error sending start message:", err)
		}
	}
}

func GetGameState(sessionID string) (GameState, error) {
	mu.RLock()
	defer mu.RUnlock()
	return GameState{}, errors.New("invalid session id")
}

func GetPlayerState(sessionID, playerID string) (PlayerState, error) {
	mu.RLock()
	defer mu.RUnlock()
	return PlayerState{}, errors.New("invalid session id")
}

func PlayerInSession(sessionID string, player *Player) bool {
	mu.Lock()
	defer mu.Unlock()
	session, exists := gameSessions[sessionID]
	if exists {
		if p, ok := session.Players[player.ID]; ok {
			return p == player
		}
		return false
	}
	return false
}

func PlayerJoin(sessionID string, player *Player) error {
	mu.Lock()
	defer mu.Unlock()
	session, exists := gameSessions[sessionID]
	if exists {
		if p, ok := session.Players[player.ID]; ok {
			if p != nil {
				return errors.New("player still in session")
			}
			session.Players[player.ID] = player
			return nil
		}
		return errors.New("player id not in the session")
	}
	return errors.New("invalid session id")
}

func PlayerLeave(sessionID, playerID string) error {
	mu.Lock()
	defer mu.Unlock()
	session, exists := gameSessions[sessionID]
	if exists {
		session.Players[playerID] = nil
		return nil
	}
	return errors.New("invalid session id")
}

func ProcessFenMove(sessionID, playerID, fenMove string) {
}
