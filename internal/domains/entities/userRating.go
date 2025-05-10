package entities

type UserRating struct {
	UserId       string  `dynamodbav:"UserId"`
	Username     string  `dynamodbav:"Username"`
	PartitionKey string  `dynamodbav:"PartitionKey"`
	Rating       float64 `dynamodbav:"Rating"`
	RD           float64 `dynamodbav:"RD"`
}
