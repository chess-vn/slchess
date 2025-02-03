package server

import "errors"

var (
	ErrStatusInvalidMove     string = "INVALID_MOVE"
	ErrStatusInvalidPlayerId string = "INVALID_PLAYER_ID"
	ErrStatusWrongTurn       string = "WRONG_TURN"
)

var ErrLoadSessionFailure = errors.New("failed to load session")
