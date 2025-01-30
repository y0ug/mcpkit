package mcpkit

import (
	"context"
	"log/slog"

	"github.com/y0ug/mcpkit/internal/client"
)

type Client = client.Client

func NewClient(
	ctx context.Context,
	logger *slog.Logger,
	serverCmd string,
	args ...string,
) (Client, error) {
	return client.New(ctx, logger, serverCmd, args...)
}
