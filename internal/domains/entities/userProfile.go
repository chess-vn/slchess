package entities

import "time"

type UserProfile struct {
	UserId     string    `dynamodbav:"UserId"`
	Username   string    `dynamodbav:"Username"`
	Picture    string    `dynamodbav:"Picture"`
	Phone      string    `dynamodbav:"Phone"`
	Locale     string    `dynamodbav:"Locale"`
	Membership string    `dynamodbav:"Membership"`
	CreatedAt  time.Time `dynamodbav:"CreatedAt"`
}
