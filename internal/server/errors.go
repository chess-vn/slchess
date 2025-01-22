package server

import "errors"

var ErrStatusInvalidMove string = "INVALID_MOVE"

var ErrLoadSessionFailure = errors.New("failed to load session")
