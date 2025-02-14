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
	id      string
	players []*player
	game    *game
	moveCh  chan move
	timer   *time.Timer
	startAt time.Time
	config  SessionConfig

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

func (session *Session) start() {
	for move := range session.moveCh {
		player, exist := session.getPlayerWithHandler(move.playerHanlder)
		if !exist {
			player.Conn.WriteJSON(errorResponse{
				Type:  "error",
				Error: ErrStatusInvalidPlayerId,
			})
			continue
		}
		switch move.control {
		case RESIGNAION:
			session.game.Resign(player.color())
		case DRAW_OFFER:
		case AGREEMENT:
			session.game.Draw(chess.DrawOffer)
		default:
			if player.Handler != session.getCurrentTurnPlayer().Handler {
				player.Conn.WriteJSON(errorResponse{
					Type:  "error",
					Error: ErrStatusWrongTurn,
				})
				continue
			}
			err := session.game.MoveStr(move.uci)
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
				session.game.outOfTime(player.Side)
				logging.Info("out of time", zap.String("player_id", player.Handler))
			} else {
				// else next turn
				currentTurnPlayer := session.getCurrentTurnPlayer()
				currentTurnPlayer.TurnStartedAt = time.Now()
				session.setTimer(currentTurnPlayer.Clock)
				logging.Info("new turn",
					zap.String("player_id", currentTurnPlayer.Handler),
					zap.String("clock_w", session.players[0].Clock.String()),
					zap.String("clock_b", session.players[1].Clock.String()),
				)
			}
		}

		session.notifyPlayers(gameStateResponse{
			Outcome: session.game.outcome().String(),
			Method:  session.game.method(),
			Fen:     session.game.FEN(),
			Clocks:  []time.Duration{session.players[0].Clock, session.players[1].Clock},
		})

		if session.game.Outcome() == chess.NoOutcome {
			session.save()
		} else {
			logging.Info("Game end by outcome",
				zap.String("outcome", session.game.Outcome().String()),
				zap.String("method", session.game.method()),
			)
			session.end()
		}
	}
}

func (s *Session) notifyPlayers(resp gameStateResponse) {
	for _, player := range s.players {
		if player.Conn == nil {
			continue
		}
		err := player.Conn.WriteJSON(sessionResponse{
			Type:      "session",
			GameState: resp,
		})
		if err != nil {
			logging.Error("couldn't notify player: ", zap.String("player_id", player.Handler))
		}
	}
}

func (s *Session) getPlayerWithHandler(handler string) (*player, bool) {
	for _, player := range s.players {
		if player.Handler == handler {
			return player, true
		}
	}
	return nil, false
}

func (s *Session) getCurrentTurnPlayer() *player {
	if s.game.Position().Turn() == chess.White {
		return s.players[0]
	}
	return s.players[1]
}

func (s *Session) processMove(playerHandler, moveUci string) {
	s.moveCh <- move{
		playerHanlder: playerHandler,
		uci:           moveUci,
		control:       NONE,
	}
}

func (s *Session) processGameControl(playerHandler string, control GameControl) {
	s.moveCh <- move{
		playerHanlder: playerHandler,
		control:       control,
	}
}

func (s *Session) save() {
	s.saveGameHandler(s)
}

func (s *Session) end() {
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
	for _, player := range s.players {
		if player.Conn != nil {
			player.Conn.Close()
		}
	}
	s.endGameHandler(s)
}

func (s *Session) isEnded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ended
}

// setTimer method    set the timer to the specified duration before trigger end game handler
func (s *Session) setTimer(d time.Duration) {
	if s.timer != nil {
		s.timer.Reset(d)
		logging.Info("clock reset", zap.String("session_id", s.id), zap.String("duration", d.String()))
		return
	}
	s.timer = time.NewTimer(d)
	go func() {
		<-s.timer.C
		s.end()
	}()
	logging.Info("clock set", zap.String("session_id", s.id), zap.String("duration", d.String()))
}

// skipTimer method    skips timer by set timer to 0 duration timeout
func (s *Session) skipTimer() {
	if s.timer == nil {
		return
	}
	s.timer.Reset(0)
	logging.Info("clock skipped", zap.String("session_id", s.id))
}

func configForGameMode(gameMode string) SessionConfig {
	switch gameMode {
	case "10min":
		return SessionConfig{
			MatchDuration: 10 * time.Minute,
			CancelTimeout: 30 * time.Second,
		}
	default:
		return SessionConfig{
			MatchDuration: 10 * time.Minute,
			CancelTimeout: 30 * time.Second,
		}
	}
}
