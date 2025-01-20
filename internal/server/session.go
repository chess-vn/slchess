package server

import (
	"errors"

	"github.com/bucket-sort/slchess/pkg/logging"
	"github.com/gorilla/websocket"
	"github.com/notnil/chess"
	"go.uber.org/zap"
)

type Session struct {
	Players map[string]*Player
	Game    *chess.Game
	MoveCh  chan move
}

type Player struct {
	Conn *websocket.Conn
	Id   string `json:"id"`
}

type move struct {
	playerId string
	uci      string
}

type gameStateResponse struct {
	Outcome string `json:"outcome"`
	Method  string `json:"method"`
	Fen     string `json:"fen"`
}

type sessionResponse struct {
	Type      string            `json:"type"`
	GameState gameStateResponse `json:"game_state"`
}

type errorResponse struct {
	Type  string `json:"type"`
	Error string `json:"error"`
}

func (session *Session) Start() {
	for move := range session.MoveCh {
		err := session.Game.MoveStr(move.uci)
		if err != nil {
			session.Players[move.playerId].Conn.WriteJSON(errorResponse{
				Type:  "error",
				Error: ErrStatusInvalidMove,
			})
			return
		}

		gameStateResp := gameStateResponse{
			Outcome: session.Game.Outcome().String(),
			Method:  session.Game.Method().String(),
			Fen:     session.Game.FEN(),
		}
		for _, player := range session.Players {
			if player == nil {
				continue
			}

			err := player.Conn.WriteJSON(sessionResponse{
				Type:      "session",
				GameState: gameStateResp,
			})
			if err != nil {
				logging.Error("couldn't notify player: ", zap.String("player_id", move.playerId))
			}
		}

		if session.Game.Outcome() != chess.NoOutcome {
			session.End()
		}

	}
}

func (s *Session) Stop() {
}

func (s *Session) End() {
}

func (s *Session) PlayerJoin(player *Player) error {
	if p, ok := s.Players[player.Id]; ok {
		if p != nil {
			return errors.New("player still in session")
		}
		s.Players[player.Id] = player
		return nil
	}
	return errors.New("player id not in the session")
}

func (s *Session) PlayerLeave(playerID string) {
	s.Players[playerID] = nil
}

func (s *Session) ProcessMove(playerId, moveUci string) {
	s.MoveCh <- move{playerId, moveUci}
}
