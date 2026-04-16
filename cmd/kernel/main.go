package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/discord-bot-framework/kernel-go/internal/commands"
	discordx "github.com/discord-bot-framework/kernel-go/internal/discord"
	"github.com/discord-bot-framework/kernel-go/internal/utils"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/sharding"
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

	paths, err := utils.NewPaths(baseDir)
	if err != nil {
		logger.Error("failed to resolve paths", "err", err)
		os.Exit(1)
	}

	shardStates, ok := utils.LoadShardStates(paths.FlagDir)
	if ok {
		logger.Info("loaded shard states", "count", len(shardStates))
	}

	var readyOnce sync.Once

	client, err := disgo.New(cfg.Token,
		bot.WithLogger(logger),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagGuilds)),
		bot.WithDefaultShardManager(),
		bot.WithShardManagerConfigOpts(
			sharding.WithLogger(logger),
			sharding.WithAutoScaling(true),
			sharding.WithShardIDsWithStates(shardStates),
			sharding.WithGatewayConfigOpts(
				gateway.WithIntents(
					gateway.IntentGuilds,
					gateway.IntentGuildMessages,
				),
				gateway.WithEnableResumeURL(true),
				gateway.WithPresenceOpts(
					gateway.WithOnlineStatus(discord.OnlineStatusOnline),
					gateway.WithListeningActivity("with disgo"),
				),
			),
		),
		bot.WithEventListeners(
			router,
			bot.NewListenerFunc(func(e *events.Ready) {
				logger.Info("gateway ready", "user_id", e.User.ID.String())
				readyOnce.Do(func() {
					err := utils.WriteReadyFlag(paths.FlagDir, os.Getpid())
					if err != nil {
						logger.Error("failed to write ready flag", "err", err)
					}
				})
			}),
		),
	)
	if err != nil {
		logger.Error("failed to build disgo client", "err", err)
		os.Exit(1)
	}
	defer client.Close(context.Background())

	state.SetShardStateSaver(func() {
		if states, err := utils.ShardStatesFromManager(client.ShardManager); err == nil {
			err := utils.SaveShardStates(paths.FlagDir, states)
			if err != nil {
				logger.Error("failed to save shard states", "err", err)
			}
		} else {
			logger.Error("failed to read shard states", "err", err)
		}
	})

	if err := commands.SyncCommands(ctx, logger, cfg, client, state); err != nil {
		logger.Error("failed to sync commands", "err", err)
		os.Exit(1)
	}

	if err := client.OpenShardManager(ctx); err != nil {
		logger.Error("failed to connect shard manager", "err", err)
		os.Exit(1)
	}

	select {
	case <-ctx.Done():
	case <-state.ShutdownRequested():
		cancel()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	state.SaveShardStates()

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
