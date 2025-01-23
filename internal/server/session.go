package server

import (
	"sync"
	"time"

	"github.com/bucket-sort/slchess/pkg/logging"
	"github.com/bucket-sort/slchess/pkg/utils"
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

	ended bool
	mu    sync.Mutex
}

type SessionConfig struct {
	MatchDuration time.Duration
	CancelTimeout time.Duration
	MaxLatency    time.Duration
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
				logging.Info("out of time", zap.String("player_id", player.Id))
				session.notifyPlayers(gameStateResponse{
					Outcome: player.Outcome().String(),
					Method:  "OUT_OF_TIME",
					Fen:     session.Game.FEN(),
					Clocks:  []time.Duration{session.Players[0].Clock, session.Players[1].Clock},
				})
				session.End()
				return
			}
			currentTurnPlayer := session.GetCurrentTurnPlayer()
			currentTurnPlayer.TurnStartedAt = time.Now()
			session.SetPlayer(currentTurnPlayer)
			session.setTimer(currentTurnPlayer.Clock)
		}

		session.notifyPlayers(gameStateResponse{
			Outcome: session.Game.Outcome().String(),
			Method:  session.Game.Method().String(),
			Fen:     session.Game.FEN(),
			Clocks:  []time.Duration{session.Players[0].Clock, session.Players[1].Clock},
		})

		if session.Game.Outcome() == chess.NoOutcome {
			session.Save()
		} else {
			logging.Info("Game end by outcome", zap.String("outcome", session.Game.Outcome().String()))
			session.End()
		}
	}
}

func (s *Session) notifyPlayers(resp gameStateResponse) {
	for _, player := range s.Players {
		if player.Conn == nil {
			continue
		}
		err := player.Conn.WriteJSON(sessionResponse{
			Type:      "session",
			GameState: resp,
		})
		if err != nil {
			logging.Error("couldn't notify player: ", zap.String("player_id", player.Id))
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.ended = true
	if !utils.IsClosed(s.moveCh) {
		close(s.moveCh)
	}
	// Fire off the timer to remove end game handling job
	s.skipTimer()
	for _, player := range s.Players {
		if player.Conn != nil {
			player.Conn.Close()
		}
	}
	s.EndGameHandler(s)
}

func (s *Session) Ended() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ended
}

// setTimer method    set the timer to the specified duration before trigger end game handler
func (s *Session) setTimer(d time.Duration) {
	if s.Timer != nil {
		s.Timer.Reset(d)
		logging.Info("clock reset", zap.String("session_id", s.Id), zap.String("duration", d.String()))
		return
	}
	s.Timer = time.NewTimer(d)
	go func() {
		<-s.Timer.C
		s.End()
	}()
	logging.Info("clock set", zap.String("session_id", s.Id), zap.String("duration", d.String()))
}

// skipTimer method    skips timer by set timer to 0 duration timeout
func (s *Session) skipTimer() {
	if s.Timer == nil {
		return
	}
	s.Timer.Reset(0)
	logging.Info("clock skipped", zap.String("session_id", s.Id))
}
