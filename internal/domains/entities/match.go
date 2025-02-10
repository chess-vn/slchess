package entities

import "time"

type Ticket struct {
	UserId    string
	MinRating int
	MaxRating int
}

type Match struct {
	Id        string
	Player1Id string
	Player2Id string
	Mode      string
	Server    string
	CreatedAt time.Time
}
