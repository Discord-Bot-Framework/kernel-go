package commands

import (
	"context"
	"log/slog"

	"github.com/discord-bot-framework/kernel-go/internal/utils"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/snowflake/v2"
)

func SyncCommands(
	ctx context.Context,
	logger *slog.Logger,
	cfg utils.Config,
	client *bot.Client,
	state *State,
) error {
	_ = ctx

	app, err := client.Rest.GetCurrentApplication()
	if err != nil {
		return err
	}

	appID := app.ID

	defs := state.CommandDefinitions()
	if cfg.GuildID != 0 {
		_, err = client.Rest.SetGuildCommands(appID, snowflake.ID(cfg.GuildID), defs)
		if err != nil {
			return err
		}

		logger.Info("synced guild commands", "guild_id", cfg.GuildID, "count", len(defs))

		return nil
	}

	_, err = client.Rest.SetGlobalCommands(appID, defs)
	if err != nil {
		return err
	}

	logger.Info("synced global commands", "count", len(defs))

	return nil
}
