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

	ABORT GameControl = iota
	RESIGN
	OFFER_DRAW
	NONE

	BLACK_OUT_OF_TIME = "BLACK_OUT_OF_TIME"
	WHITE_OUT_OF_TIME = "WHITE_OUT_OF_TIME"
)

type game struct {
	chess.Game
	customOutcome chess.Outcome
	drawOffers    []bool
	moves         []move
}

func newGame() *game {
	g := chess.NewGame(
		chess.UseNotation(chess.UCINotation{}),
	)
	return &game{
		Game:       *g,
		drawOffers: []bool{false, false},
		moves:      []move{},
	}
}

func restoreGame(gameState string) (*game, error) {
	withFen, err := chess.FEN(gameState)
	if err != nil {
		return nil, err
	}
	g := chess.NewGame(
		withFen,
		chess.UseNotation(chess.UCINotation{}),
	)
	return &game{
		Game:       *g,
		drawOffers: []bool{false, false},
		moves:      []move{},
	}, nil
}

func (g *game) OfferDraw(side chess.Color) bool {
	switch side {
	case chess.White:
		g.drawOffers[0] = true
	case chess.Black:
		g.drawOffers[1] = true
	}
	if g.drawOffers[0] && g.drawOffers[1] {
		g.Draw(chess.DrawOffer)
		return true
	}
	return false
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

func (g *game) lastMove() move {
	if length := len(g.moves); length > 0 {
		return g.moves[length-1]
	}
	return move{}
}

func (g *game) move(move move) error {
	if err := g.MoveStr(move.uci); err != nil {
		return err
	}
	g.moves = append(g.moves, move)
	return nil
}

type move struct {
	playerId  string
	uci       string
	control   GameControl
	createdAt time.Time
}

type player struct {
	Id            string
	Rating        float64
	RD            float64
	NewRatings    []float64
	NewRDs        []float64
	Conn          *websocket.Conn
	Side          Side
	Status        Status
	Clock         time.Duration
	TurnStartedAt time.Time
}

func newPlayer(
	conn *websocket.Conn,
	playerId string,
	side Side,
	clock time.Duration,
	rating float64,
	rd float64,
	newRatings []float64,
	newRDs []float64,
) player {
	player := player{
		Id:         playerId,
		Rating:     rating,
		RD:         rd,
		NewRatings: newRatings,
		NewRDs:     newRDs,
		Conn:       conn,
		Side:       side,
		Status:     INIT,
		Clock:      clock,
	}
	return player
}

func (p *player) color() chess.Color {
	if p.Side == WHITE_SIDE {
		return chess.White
	}
	return chess.Black
}

func (p *player) updateClock(
	timeTaken time.Duration,
	lagForgiven time.Duration,
	increment time.Duration,
) {
	p.Clock = p.Clock - timeTaken + lagForgiven + increment
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
