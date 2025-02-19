package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/chess-vn/slchess/pkg/logging"
	"github.com/chess-vn/slchess/pkg/utils"
	"github.com/notnil/chess"
	"go.uber.org/zap"
)

type Match struct {
	id      string
	players []*player
	game    *game
	moveCh  chan move
	timer   *time.Timer
	startAt time.Time
	config  MatchConfig

	endGameHandler  func(*Match)
	saveGameHandler func(*Match)

	ended bool
	mu    sync.Mutex
}

type MatchConfig struct {
	MatchDuration time.Duration
	CancelTimeout time.Duration
}

type matchResponse struct {
	Type      string            `json:"type"`
	GameState gameStateResponse `json:"game"`
}

type gameStateResponse struct {
	Outcome string   `json:"outcome"`
	Method  string   `json:"method"`
	Fen     string   `json:"fen"`
	Clocks  []string `json:"clocks"`
}

type errorResponse struct {
	Type  string `json:"type"`
	Error string `json:"error"`
}

func (match *Match) start() {
	for move := range match.moveCh {
		player, exist := match.getPlayerWithId(move.playerId)
		if !exist {
			player.Conn.WriteJSON(errorResponse{
				Type:  "error",
				Error: ErrStatusInvalidPlayerId,
			})
			continue
		}
		switch move.control {
		case RESIGNAION:
			match.game.Resign(player.color())
		case DRAW_OFFER:
		case AGREEMENT:
			match.game.Draw(chess.DrawOffer)
		default:
			if expectedId := match.getCurrentTurnPlayer().Id; player.Id != expectedId {
				player.Conn.WriteJSON(errorResponse{
					Type:  "error",
					Error: fmt.Sprintf("%s: want %s - got %s", ErrStatusWrongTurn, expectedId, player.Id),
				})
				continue
			}
			err := match.game.MoveStr(move.uci)
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
				match.game.outOfTime(player.Side)
				logging.Info("out of time", zap.String("player_id", player.Id))
			} else {
				// else next turn
				currentTurnPlayer := match.getCurrentTurnPlayer()
				currentTurnPlayer.TurnStartedAt = time.Now()
				match.setTimer(currentTurnPlayer.Clock)
				logging.Info("new turn",
					zap.String("player_id", currentTurnPlayer.Id),
					zap.String("clock_w", match.players[0].Clock.String()),
					zap.String("clock_b", match.players[1].Clock.String()),
				)
			}
		}

		match.notifyPlayers(gameStateResponse{
			Outcome: match.game.outcome().String(),
			Method:  match.game.method(),
			Fen:     match.game.FEN(),
			Clocks:  []string{match.players[0].Clock.String(), match.players[1].Clock.String()},
		})

		// Save game state
		match.save()

		// Check if game ended
		if match.game.Outcome() != chess.NoOutcome {
			logging.Info("Game end by outcome",
				zap.String("outcome", match.game.Outcome().String()),
				zap.String("method", match.game.method()),
			)
			match.end()
		}
	}
}

func (m *Match) notifyPlayers(resp gameStateResponse) {
	for _, player := range m.players {
		if player == nil || player.Conn == nil {
			continue
		}
		err := player.Conn.WriteJSON(matchResponse{
			Type:      "game_state",
			GameState: resp,
		})
		if err != nil {
			logging.Error("couldn't notify player: ", zap.String("player_id", player.Id))
		}
	}
}

func (m *Match) getPlayerWithId(id string) (*player, bool) {
	for _, player := range m.players {
		if player.Id == id {
			return player, true
		}
	}
	return nil, false
}

func (m *Match) getCurrentTurnPlayer() *player {
	if m.game.Position().Turn() == chess.White {
		return m.players[0]
	}
	return m.players[1]
}

func (m *Match) processMove(playerId, moveUci string) {
	m.moveCh <- move{
		playerId: playerId,
		uci:      moveUci,
		control:  NONE,
	}
}

func (m *Match) processGameControl(playerId string, control GameControl) {
	m.moveCh <- move{
		playerId: playerId,
		control:  control,
	}
}

func (m *Match) save() {
	m.saveGameHandler(m)
}

func (m *Match) end() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ended {
		return
	}
	m.ended = true
	if !utils.IsClosed(m.moveCh) {
		close(m.moveCh)
	}
	// Fire off the timer to remove end game handling job
	m.skipTimer()
	for _, player := range m.players {
		if player.Conn != nil {
			player.Conn.Close()
		}
	}
	m.endGameHandler(m)
}

func (m *Match) isEnded() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ended
}

// setTimer method    set the timer to the specified duration before trigger end game handler
func (m *Match) setTimer(d time.Duration) {
	if m.timer != nil {
		m.timer.Reset(d)
		logging.Info("clock reset", zap.String("match_id", m.id), zap.String("duration", d.String()))
		return
	}
	m.timer = time.NewTimer(d)
	go func() {
		<-m.timer.C
		m.end()
	}()
	logging.Info("clock set", zap.String("match_id", m.id), zap.String("duration", d.String()))
}

// skipTimer method    skips timer by set timer to 0 duration timeout
func (m *Match) skipTimer() {
	if m.timer == nil {
		return
	}
	m.timer.Reset(0)
	logging.Info("clock skipped", zap.String("match_id", m.id))
}

func configForGameMode(gameMode string) MatchConfig {
	switch gameMode {
	case "10min":
		return MatchConfig{
			MatchDuration: 10 * time.Minute,
			CancelTimeout: 30 * time.Second,
		}
	default:
		return MatchConfig{
			MatchDuration: 10 * time.Minute,
			CancelTimeout: 30 * time.Second,
		}
	}
}

func (m *Match) calculatePlayerRatings() ([]float64, error) {
	switch m.game.outcome() {
	case chess.WhiteWon:
		return []float64{m.players[0].Rating + m.players[0].RatingChanges[0], m.players[1].Rating + m.players[1].RatingChanges[2]}, nil
	case chess.BlackWon:
		return []float64{m.players[0].Rating + m.players[0].RatingChanges[2], m.players[1].Rating + m.players[1].RatingChanges[0]}, nil
	case chess.Draw:
		return []float64{m.players[0].Rating + m.players[0].RatingChanges[1], m.players[1].Rating + m.players[1].RatingChanges[1]}, nil
	}
	return nil, ErrGameNotEnded
}
