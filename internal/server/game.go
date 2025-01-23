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
)

type Move struct {
	playerId  string
	uci       string
	control   GameControl
	createdAt time.Time
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

func (p *Player) Outcome() chess.Outcome {
	if p.Side == WHITE_SIDE {
		return chess.WhiteWon
	}
	return chess.BlackWon
}
