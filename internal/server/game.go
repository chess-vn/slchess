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

type Game struct {
	chess.Game
	customOutcome chess.Outcome
}

func NewGame() *Game {
	g := chess.NewGame(chess.UseNotation(chess.UCINotation{}))
	return &Game{Game: *g}
}

func (g *Game) OutOfTime(side Side) {
	if side == WHITE_SIDE {
		g.customOutcome = WHITE_OUT_OF_TIME
	} else {
		g.customOutcome = BLACK_OUT_OF_TIME
	}
}

func (g *Game) CustomOutcome() chess.Outcome {
	switch g.customOutcome {
	case BLACK_OUT_OF_TIME:
		return chess.WhiteWon
	case WHITE_OUT_OF_TIME:
		return chess.BlackWon
	default:
		return g.Outcome()
	}
}

func (g *Game) CustomMethodString() string {
	switch g.customOutcome {
	case WHITE_OUT_OF_TIME, BLACK_OUT_OF_TIME:
		return "OUT_OF_TIME"
	default:
		return g.Method().String()
	}
}

type Move struct {
	PlayerId string
	Uci      string
	Control  GameControl
}

type Player struct {
	Id            string
	Conn          *websocket.Conn
	Side          Side
	Status        Status
	Clock         time.Duration
	TurnStartedAt time.Time
}

func NewPlayer(conn *websocket.Conn, playerId string, side Side, clock time.Duration) Player {
	player := Player{
		Id:     playerId,
		Conn:   conn,
		Side:   side,
		Status: INIT,
		Clock:  clock,
	}
	return player
}

func (p *Player) Color() chess.Color {
	if p.Side == WHITE_SIDE {
		return chess.White
	}
	return chess.Black
}
