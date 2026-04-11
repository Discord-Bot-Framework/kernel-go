package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/discord-bot-framework/kernel-go/internal/commands"
	discordx "github.com/discord-bot-framework/kernel-go/internal/discord"
	"github.com/discord-bot-framework/kernel-go/internal/utils"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
		os.Interrupt,
	)
	defer cancel()

	baseDir, err := utils.ResolveBaseDir()
	if err != nil {
		os.Exit(1)
	}

	cfg, err := utils.LoadConfig(baseDir)
	if err != nil {
		os.Exit(1)
	}

	logger, closeLogger, err := utils.NewLogger(baseDir)
	if err != nil {
		os.Exit(1)
	}
	defer closeLogger()

	router := discordx.NewRouter(logger, cfg)
	state := commands.NewState(logger, cfg, baseDir)
	commands.Register(router, state)

	client, err := disgo.New(cfg.Token,
		bot.WithLogger(logger),
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuilds,
				gateway.IntentGuildMessages,
			),
		),
		bot.WithEventListeners(
			router,
			bot.NewListenerFunc(func(e *events.Ready) {
				logger.Info("gateway ready", "user_id", e.User.ID.String())
				_ = e.Client().SetPresence(context.Background(),
					gateway.WithOnlineStatus(discord.OnlineStatusOnline),
					gateway.WithListeningActivity("with disgo"),
				)
			}),
		),
	)
	if err != nil {
		logger.Error("failed to build disgo client", "err", err)
		os.Exit(1)
	}
	defer client.Close(context.Background())

	if err := commands.SyncCommands(ctx, logger, cfg, client, state); err != nil {
		logger.Error("failed to sync commands", "err", err)
		os.Exit(1)
	}

	if err := client.OpenGateway(ctx); err != nil {
		logger.Error("failed to connect gateway", "err", err)
		os.Exit(1)
	}

	select {
	case <-ctx.Done():
	case <-state.ShutdownRequested():
		cancel()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := discordx.Shutdown(
		shutdownCtx,
		logger,
		client,
		state.StopAllModules,
	); err != nil &&
		!errors.Is(err, context.Canceled) {
		logger.Error("shutdown error", "err", err)
	}

	_ = disgo.Version
}
