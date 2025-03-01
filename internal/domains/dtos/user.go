package dtos

import (
	"time"

	"github.com/chess-vn/slchess/internal/domains/entities"
)

type UserResponse struct {
	Id         string    `json:"id"`
	Username   string    `json:"username"`
	Phone      string    `json:"phone,omitempty"`
	Locale     string    `json:"locale"`
	Picture    string    `json:"picture"`
	Rating     float64   `json:"rating"`
	Membership string    `json:"membership"`
	CreatedAt  time.Time `json:"createdAt"`
}

func UserResponseFromEntities(userProfile entities.UserProfile, userRating entities.UserRating, full bool) UserResponse {
	user := UserResponse{
		Id:         userProfile.UserId,
		Username:   userProfile.Username,
		Locale:     userProfile.Locale,
		Picture:    userProfile.Picture,
		Rating:     userRating.Rating,
		Membership: userProfile.Membership,
		CreatedAt:  userProfile.CreatedAt,
	}
	if full {
		user.Phone = userProfile.Phone
	}
	return user
}
