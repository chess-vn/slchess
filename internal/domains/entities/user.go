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

type UserProfile struct {
	UserId     string    `dynamodbav:"UserId"`
	Phone      string    `dynamodbav:"Phone"`
	Locale     string    `dynamodbav:"Locale"`
	Membership string    `dynamodbav:"Membership"`
	CreatedAt  time.Time `dynamodbav:"CreatedAt"`
}

type UserRating struct {
	UserId string  `dynamodbav:"UserId"`
	Rating float64 `dynamodbav:"Rating"`
	RD     float64 `dynamodbav:"RD"`
}
