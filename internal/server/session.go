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
	Players []*Player
	StartAt time.Time
	Config  SessionConfig

	game   *Game
	moveCh chan Move
	timer  *time.Timer

	endGameHandler  func(*Session)
	saveGameHandler func(*Session)

	ended bool
	mu    sync.Mutex
}

type SessionConfig struct {
	MatchDuration time.Duration
	CancelTimeout time.Duration
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
		player, exist := session.GetPlayerWithId(move.PlayerId)
		if !exist {
			player.Conn.WriteJSON(errorResponse{
				Type:  "error",
				Error: ErrStatusInvalidPlayerId,
			})
			continue
		}
		switch move.Control {
		case RESIGNAION:
			session.game.Resign(player.Color())
		case DRAW_OFFER:
		case AGREEMENT:
			session.game.Draw(chess.DrawOffer)
		default:
			if player.Id != session.getCurrentTurnPlayer().Id {
				player.Conn.WriteJSON(errorResponse{
					Type:  "error",
					Error: ErrStatusWrongTurn,
				})
				continue
			}
			err := session.game.MoveStr(move.Uci)
			if err != nil {
				player.Conn.WriteJSON(errorResponse{
					Type:  "error",
					Error: ErrStatusInvalidMove,
				})
				continue
			}

			// If making move, update clock
			player.Clock -= time.Since(player.TurnStartedAt)
			// If clock runs out, end the game
			if player.Clock <= 0 {
				session.game.OutOfTime(player.Side)
				logging.Info("out of time", zap.String("player_id", player.Id))
			} else {
				// else next turn
				currentTurnPlayer := session.getCurrentTurnPlayer()
				currentTurnPlayer.TurnStartedAt = time.Now()
				session.SetTimer(currentTurnPlayer.Clock)
				logging.Info("new turn",
					zap.String("player_id", currentTurnPlayer.Id),
					zap.String("clock_w", session.Players[0].Clock.String()),
					zap.String("clock_b", session.Players[1].Clock.String()),
				)
			}
		}

		session.notifyPlayers(gameStateResponse{
			Outcome: session.game.CustomOutcome().String(),
			Method:  session.game.CustomMethodString(),
			Fen:     session.game.FEN(),
			Clocks:  []time.Duration{session.Players[0].Clock, session.Players[1].Clock},
		})

		if session.game.Outcome() == chess.NoOutcome {
			session.save()
		} else {
			logging.Info("Game end by outcome",
				zap.String("outcome", session.game.Outcome().String()),
				zap.String("method", session.game.CustomMethodString()),
			)
			session.End()
		}
	}
}

func (s *Session) GetPlayerWithId(playerId string) (*Player, bool) {
	for _, player := range s.Players {
		if player.Id == playerId {
			return player, true
		}
	}
	return nil, false
}

func (s *Session) getCurrentTurnPlayer() *Player {
	if s.game.Position().Turn() == chess.White {
		return s.Players[0]
	}
	return s.Players[1]
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

func (s *Session) ProcessMove(playerId, moveUci string) {
	s.moveCh <- Move{
		PlayerId: playerId,
		Uci:      moveUci,
		Control:  NONE,
	}
}

func (s *Session) ProcessGameControl(playerId string, control GameControl) {
	s.moveCh <- Move{
		PlayerId: playerId,
		Control:  control,
	}
}

func (s *Session) save() {
	s.saveGameHandler(s)
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
	s.endGameHandler(s)
}

func (s *Session) IsEnded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ended
}

// SetTimer method    set the timer to the specified duration before trigger end game handler
func (s *Session) SetTimer(d time.Duration) {
	if s.timer != nil {
		s.timer.Reset(d)
		logging.Info("clock reset", zap.String("session_id", s.Id), zap.String("duration", d.String()))
		return
	}
	s.timer = time.NewTimer(d)
	go func() {
		<-s.timer.C
		s.End()
	}()
	logging.Info("clock set", zap.String("session_id", s.Id), zap.String("duration", d.String()))
}

// skipTimer method    skips timer by set timer to 0 duration timeout
func (s *Session) skipTimer() {
	if s.timer == nil {
		return
	}
	s.timer.Reset(0)
	logging.Info("clock skipped", zap.String("session_id", s.Id))
}
