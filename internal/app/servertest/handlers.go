package servertest

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/chess-vn/slchess/internal/aws/storage"
	"github.com/chess-vn/slchess/pkg/logging"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

func (s *server) handleAbortGame(match *Match) {
	if match == nil {
		return
	}
	s.removeMatch(match.id)
	logging.Info("match aborted", zap.String("match_id", match.id))
}

// Handler for saving current game state.
func (s *server) handleSaveGame(match *Match) {
}

// Handler for when a game match ends.
func (s *server) handleEndGame(match *Match) {
	if match == nil {
		return
	}
	s.removeMatch(match.id)
	logging.Info("match ended", zap.String("match_id", match.id))
}

// Handler for when a user connection closes
func (s *server) handlePlayerDisconnect(match *Match, playerId string) {
	if match == nil {
		return
	}

	player, exist := match.getPlayerWithId(playerId)
	if !exist {
		logging.Fatal("invalid player id", zap.String("player_id", playerId))
		return
	}
	player.Conn = nil
	player.Status = DISCONNECTED

	currentClock := match.getCurrentTurnPlayer().Clock

	// If both player disconnected, set the clock to current turn clock
	if match.players[0].Status == match.players[1].Status {
		logging.Info(
			"both player disconnected",
			zap.String("match_id", match.id),
		)
		if !match.isEnded() {
			match.setTimer(currentClock)
		}
	} else {
		// Else only set the timer for the disconnected player
		logging.Info(
			"player disconnected",
			zap.String("match_id", match.id),
			zap.String("player_id", player.Id),
		)
		if !match.isEnded() {
			if currentClock < match.cfg.DisconnectTimeout {
				match.setTimer(currentClock)
			} else {
				match.setTimer(match.cfg.DisconnectTimeout)
			}
		}
		match.notifyAboutPlayerStatus(playerStatusResponse{
			Type:     "playerStatus",
			PlayerId: playerId,
			Status:   player.Status.String(),
		})
	}
}

func (s *server) handlePlayerJoin(
	conn *websocket.Conn,
	match *Match,
	playerId string,
) {
	if match == nil {
		return
	}

	player, exist := match.getPlayerWithId(playerId)
	if !exist {
		logging.Fatal("invalid player id", zap.String("player_id", playerId))
		return
	}
	if player.Status == INIT && player.Side == WHITE_SIDE {
		match.startAt = time.Now()
		player.TurnStartedAt = match.startAt
		match.setTimer(match.cfg.MatchDuration)
		err := s.storageClient.UpdateActiveMatch(
			context.Background(),
			match.id,
			storage.ActiveMatchUpdateOptions{
				StartedAt: aws.Time(match.startAt),
			},
		)
		if err != nil {
			logging.Error(
				"failed to update match: %w",
				zap.Error(err),
			)
		}
	}
	player.Conn = conn
	player.Status = CONNECTED

	match.syncPlayer(player)

	logging.Info("player connected",
		zap.String("player_id", playerId),
		zap.String("match_id", match.id),
	)

	match.notifyAboutPlayerStatus(playerStatusResponse{
		Type:     "playerStatus",
		PlayerId: playerId,
		Status:   player.Status.String(),
	})
}

// Handler for when user sends a message
func (s *server) handleWebSocketMessage(
	playerId string,
	match *Match,
	payload payload,
) {
	if match == nil {
		logging.Error("match not loaded")
		return
	}
	if time.Since(payload.CreatedAt) < 0 {
		logging.Info("invalid timestamp",
			zap.String("created_at", payload.CreatedAt.String()),
			zap.String("validate_time", time.Now().String()),
		)
		return
	}
	switch payload.Type {
	case "gameData":
		action := payload.Data["action"]
		switch action {
		case "abort":
			match.processGameControl(playerId, ABORT)
		case "resign":
			match.processGameControl(playerId, RESIGN)
		case "offerDraw":
			match.processGameControl(playerId, OFFER_DRAW)
		case "declineDraw":
			match.processGameControl(playerId, DECLINE_DRAW)
		case "move":
			match.processMove(playerId, payload.Data["move"], payload.CreatedAt)
		default:
			logging.Info("invalid game action:", zap.String("action", payload.Type))
			return
		}
		logging.Info(
			"game data",
			zap.String("match_id", match.id),
			zap.String("action", action),
		)
	case "sync":
		match.syncPlayerWithId(playerId)
	default:
		logging.Info("invalid payload type:", zap.String("type", payload.Type))
	}
}
