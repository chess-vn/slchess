package usecases

import (
	"github.com/bucket-sort/slchess/internal/domains/interfaces"
	"github.com/bucket-sort/slchess/internal/domains/models/dtos"
	"github.com/bucket-sort/slchess/internal/domains/models/entities"
)

type UserUsecase struct {
	userRepo interfaces.IUserRepository
}

func NewUserUsecase(userRepo interfaces.IUserRepository) interfaces.IUserUsecase {
	return &UserUsecase{
		userRepo: userRepo,
	}
}

func (u *UserUsecase) CreateUser(userCreateDTO dtos.UserCreateRequest) (entities.User, error) {
	user := entities.User{
		Email:     userCreateDTO.Email,
		Handler:   userCreateDTO.Handler,
		FirstName: userCreateDTO.FirstName,
		LastName:  userCreateDTO.LastName,
		Password:  userCreateDTO.Password,
	}
	err := user.Validate()
	if err != nil {
		return entities.User{}, err
	}
	user.HashPassword()

	err = u.userRepo.Create(user)
	if err != nil {
		return entities.User{}, err
	}

	return user, nil
}
