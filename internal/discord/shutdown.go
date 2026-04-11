package discord

import (
	"context"
	"log/slog"

	"github.com/disgoorg/disgo/bot"
)

func Shutdown(
	ctx context.Context,
	logger *slog.Logger,
	client *bot.Client,
	stopModules func(ctx context.Context),
) error {
	if stopModules != nil {
		stopModules(ctx)
	}

	client.Close(ctx)
	logger.Info("shutdown complete")

	return nil
}
