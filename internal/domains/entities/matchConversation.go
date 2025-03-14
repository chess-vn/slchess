package entities

type MatchConversation struct {
	MatchId        string `dynamodbav:"MatchId"`
	ConversationId string `dynamodbav:"ConversationId"`
}
