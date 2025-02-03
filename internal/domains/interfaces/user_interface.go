package interfaces

import (
	"github.com/bucket-sort/slchess/internal/domains/models/dtos"
	"github.com/bucket-sort/slchess/internal/domains/models/entities"
)

type (
	IUserUsecase interface {
		CreateUser(dtos.UserCreateRequest) (entities.User, error)
	}

	IUserRepository interface {
		Create(entities.User) error
	}
)
