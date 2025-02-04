package entities

import "time"

type Message struct {
	Id        string
	SessionId string
	SenderId  string
	Content   string
	CreatedAt time.Time
}
