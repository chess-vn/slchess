package handlers

import (
	"github.com/bucket-sort/slchess/internal/domains/interfaces"
	"github.com/bucket-sort/slchess/internal/repositories"
	"github.com/bucket-sort/slchess/internal/usecases"
)

type UserHandler struct {
	userUsecase interfaces.IUserUsecase
}

func NewUserHandler() *UserHandler {
	userRepo := repositories.NewUserRepository()
	userUsecase := usecases.NewUserUsecase(userRepo)
	return &UserHandler{
		userUsecase: userUsecase,
	}
}

func (h *UserHandler) CreateUser() {
}
