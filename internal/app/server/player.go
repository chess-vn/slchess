package server

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/notnil/chess"
)

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

	mu *sync.Mutex
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
		mu:         new(sync.Mutex),
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

func (p *player) setConn(conn *websocket.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if conn == nil {
		p.Status = DISCONNECTED
	} else {
		p.Status = CONNECTED
	}
	p.Conn = conn
}

func (p *player) writeJson(msg interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p == nil || p.Conn == nil {
		return nil
	}
	return p.Conn.WriteJSON(msg)
}

func (p *player) writeControl(messageType int, data []byte, deadline time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p == nil || p.Conn == nil {
		return nil
	}
	return p.Conn.WriteControl(messageType, data, deadline)
}
