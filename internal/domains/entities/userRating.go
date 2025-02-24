package entities

type UserRating struct {
	UserId string  `dynamodbav:"UserId"`
	Rating float64 `dynamodbav:"Rating"`
	RD     float64 `dynamodbav:"RD"`
}
