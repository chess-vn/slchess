package entities

import "time"

type Ticket struct {
	UserId    string
	MinRating int
	MaxRating int
}

type Match struct {
	Id        string
	Player1   string
	Player2   string
	GameMode  string
	Server    string
	CreatedAt time.Time
}
