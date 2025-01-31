package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/y0ug/mcpkit"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	srv := mcpkit.NewServer(logger)
	if err := srv.Serve(context.Background()); err != nil {
		logger.Error("Server exited with error", "error", err)
		os.Exit(1)
	}
}
