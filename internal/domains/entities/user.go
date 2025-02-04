package entities

import (
	"time"
)

type User struct {
	Id         string
	Email      string
	Password   string
	Username   string
	Avatar     string
	Country    string
	Elo        uint16
	Membership string
	CreatedAt  time.Time
}
