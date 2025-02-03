package main

import (
	"github.com/bucket-sort/slchess/internal/app/server"
	"github.com/bucket-sort/slchess/pkg/logging"
	"go.uber.org/zap"
)

func main() {
	logging.Fatal("Game server exited: ", zap.Error(
		server.NewServer().Start(),
	))
}
