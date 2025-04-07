package main

import (
	"github.com/chess-vn/slchess/internal/app/servertest"
	"github.com/chess-vn/slchess/pkg/logging"
	"go.uber.org/zap"
)

func main() {
	logging.Fatal("Game server exited: ", zap.Error(
		servertest.NewServer().Start(),
	))
}
