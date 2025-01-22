package server

import (
	"time"

	"github.com/bucket-sort/slchess/pkg/logging"
	"github.com/notnil/chess"
	"go.uber.org/zap"
)

type Session struct {
	Id      string
	Players []Player
	Game    *chess.Game
	moveCh  chan Move
	Timer   *time.Timer
	StartAt time.Time
	Config  SessionConfig

	EndGameHandler  func(*Session)
	SaveGameHandler func(*Session)
}

type SessionConfig struct {
	MatchDuration time.Duration
}

type sessionResponse struct {
	Type      string            `json:"type"`
	GameState gameStateResponse `json:"game"`
}

type gameStateResponse struct {
	Outcome string          `json:"outcome"`
	Method  string          `json:"method"`
	Fen     string          `json:"fen"`
	Clocks  []time.Duration `json:"clocks"`
}

type errorResponse struct {
	Type  string `json:"type"`
	Error string `json:"error"`
}

func (session *Session) Start() {
	for move := range session.moveCh {
		switch move.control {
		case RESIGNAION:
			player, exist := session.GetPlayerWithId(move.playerId)
			if !exist {
				player.Conn.WriteJSON(errorResponse{
					Type:  "error",
					Error: ErrStatusInvalidPlayerId,
				})
				continue
			}
			session.Game.Resign(player.Color())
		case DRAW_OFFER:
		case AGREEMENT:
			session.Game.Draw(chess.DrawOffer)
		default:
			player := session.GetCurrentTurnPlayer()
			if move.playerId != player.Id {
				player.Conn.WriteJSON(errorResponse{
					Type:  "error",
					Error: ErrStatusInvalidPlayerId,
				})
				continue
			}
			err := session.Game.MoveStr(move.uci)
			if err != nil {
				player.Conn.WriteJSON(errorResponse{
					Type:  "error",
					Error: ErrStatusInvalidMove,
				})
				continue
			}

			// If making move, update clock
			player.Clock -= move.createdAt.Sub(player.TurnStartedAt)
			if player.Clock <= 0 {
				session.End()
				continue
			}
			currentTurnPlayer := session.GetCurrentTurnPlayer()
			currentTurnPlayer.TurnStartedAt = time.Now()
			session.SetPlayer(currentTurnPlayer)
			session.setTimer(currentTurnPlayer.Clock)
		}

		gameStateResp := gameStateResponse{
			Outcome: session.Game.Outcome().String(),
			Method:  session.Game.Method().String(),
			Fen:     session.Game.FEN(),
			Clocks:  []time.Duration{session.Players[0].Clock, session.Players[1].Clock},
		}
		for _, player := range session.Players {
			if player.Conn == nil {
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

		if session.Game.Outcome() == chess.NoOutcome {
			session.Save()
		} else {
			session.End()
		}
	}
}

func (s *Session) SetPlayer(player Player) bool {
	for i, p := range s.Players {
		if player.Id == p.Id {
			s.Players[i] = player
			return true
		}
	}
	return false
}

func (s *Session) GetPlayerWithId(playerId string) (Player, bool) {
	for _, player := range s.Players {
		if player.Id == playerId {
			return player, true
		}
	}
	return Player{}, false
}

func (s *Session) GetCurrentTurnPlayer() Player {
	if s.Game.Position().Turn() == chess.White {
		return s.Players[0]
	}
	return s.Players[1]
}

func (s *Session) ProcessMove(playerId, moveUci string, createdAt time.Time) {
	s.moveCh <- Move{playerId, moveUci, NONE, createdAt}
}

func (s *Session) ProcessGameControl(playerId string, control GameControl, createdAt time.Time) {
	s.moveCh <- Move{playerId, "", control, createdAt}
}

func (s *Session) Save() {
	s.SaveGameHandler(s)
}

func (s *Session) End() {
	close(s.moveCh)
	s.Timer.Stop()
	s.EndGameHandler(s)
}

// setTimer method    set the timer to the specified duration before trigger end game handler
func (s *Session) setTimer(d time.Duration) {
	if s.Timer != nil {
		s.Timer.Reset(d)
		return
	}
	s.Timer = time.NewTimer(d)
	go func() {
		<-s.Timer.C
		s.End()
	}()
}
