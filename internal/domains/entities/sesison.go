package entities

import "time"

type Session struct {
	Id        string
	Player1Id string
	Player2Id string
	Server    string
	CreatedAt time.Time
}
