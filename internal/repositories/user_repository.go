package repositories

import (
	"github.com/bucket-sort/slchess/internal/domains/interfaces"
	"github.com/bucket-sort/slchess/internal/domains/models/entities"
)

type userRepository struct{}

func NewUserRepository() interfaces.IUserRepository {
	return &userRepository{}
}

func (r *userRepository) Create(user entities.User) error {
		return nil
}
