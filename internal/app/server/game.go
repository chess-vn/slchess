package server

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/notnil/chess"
)

type (
	Status      uint8
	Side        bool
	GameControl uint8
)

const (
	INIT Status = iota
	CONNECTED
	DISCONNECTED

	WHITE_SIDE Side = true
	BLACK_SIDE Side = false

	RESIGNAION GameControl = iota
	DRAW_OFFER
	AGREEMENT
	NONE

	BLACK_OUT_OF_TIME = "BLACK_OUT_OF_TIME"
	WHITE_OUT_OF_TIME = "WHITE_OUT_OF_TIME"
)

type game struct {
	chess.Game
	customOutcome chess.Outcome
}

func newGame() *game {
	g := chess.NewGame(chess.UseNotation(chess.UCINotation{}))
	return &game{Game: *g}
}

func (g *game) outOfTime(side Side) {
	if side == WHITE_SIDE {
		g.customOutcome = WHITE_OUT_OF_TIME
	} else {
		g.customOutcome = BLACK_OUT_OF_TIME
	}
}

func (g *game) outcome() chess.Outcome {
	switch g.customOutcome {
	case BLACK_OUT_OF_TIME:
		return chess.WhiteWon
	case WHITE_OUT_OF_TIME:
		return chess.BlackWon
	default:
		return g.Outcome()
	}
}

func (g *game) method() string {
	switch g.customOutcome {
	case WHITE_OUT_OF_TIME, BLACK_OUT_OF_TIME:
		return "OUT_OF_TIME"
	default:
		return g.Method().String()
	}
}

type move struct {
	playerId string
	uci      string
	control  GameControl
}

type player struct {
	Id            string
	Rating        float64
	RatingChanges []float64
	Conn          *websocket.Conn
	Side          Side
	Status        Status
	Clock         time.Duration
	TurnStartedAt time.Time
}

func newPlayer(conn *websocket.Conn, playerId string, side Side, clock time.Duration, rating float64, ratingChanges []float64) player {
	player := player{
		Id:            playerId,
		Rating:        rating,
		RatingChanges: ratingChanges,
		Conn:          conn,
		Side:          side,
		Status:        INIT,
		Clock:         clock,
	}
	return player
}

func (p *player) color() chess.Color {
	if p.Side == WHITE_SIDE {
		return chess.White
	}
	return chess.Black
}

func (s Status) String() string {
	switch s {
	case INIT:
		return "INIT"
	case CONNECTED:
		return "CONNECTED"
	case DISCONNECTED:
		return "DISCONNECTED"
	default:
		return "UNKNOWN"
	}
}
