package entities

import "time"

type Friendship struct {
	UserId         string    `dynamodb:"UserId"`
	FriendId       string    `dynamodb:"FriendId"`
	ConversationId string    `dynamodb:"ConversationId"`
	Status         string    `dynamodb:"Status"`
	StartedAt      time.Time `dynamodb:"StartedAt"`
}
