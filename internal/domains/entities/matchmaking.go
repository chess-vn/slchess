package entities

type MatchmakingTicket struct {
	UserId    string  `dynamodbav:"UserId"`
	MinRating float64 `dynamodbav:"MinRating"`
	MaxRating float64 `dynamodbav:"MaxRating"`
	GameMode  string  `dynamodbav:"GameMode"`
}
