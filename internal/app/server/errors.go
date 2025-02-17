package server

import "errors"

var (
	ErrStatusInvalidMove          string = "INVALID_MOVE"
	ErrStatusInvalidPlayerHandler string = "INVALID_PLAYER_HANDLER"
	ErrStatusWrongTurn            string = "WRONG_TURN"
)

var ErrFailedToLoadMatch = errors.New("failed to load match")
