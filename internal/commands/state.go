package commands

import (
	"context"
	"log/slog"
	"sync"

	"github.com/discord-bot-framework/kernel-go/internal/modules"
	"github.com/discord-bot-framework/kernel-go/internal/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

type State struct {
	logger *slog.Logger

	modules *modules.Manager

	shutdownCh chan struct{}
	paths      utils.Paths

	cfg utils.Config

	shutdownOnce sync.Once
}

func NewState(logger *slog.Logger, cfg utils.Config, baseDir string) *State {
	p, _ := utils.NewPaths(baseDir)

	return &State{
		logger:     logger,
		cfg:        cfg,
		paths:      p,
		modules:    modules.NewManager(logger, cfg, p),
		shutdownCh: make(chan struct{}),
	}
}

func (s *State) RequestShutdown(reason string) {
	s.shutdownOnce.Do(func() {
		s.logger.Info("shutdown requested", "reason", reason)
		close(s.shutdownCh)
	})
}

func (s *State) ShutdownRequested() <-chan struct{} {
	return s.shutdownCh
}

func (s *State) StopAllModules(ctx context.Context) {
	s.modules.StopAll(ctx)
}

func (s *State) Authorize(interaction discord.Interaction) bool {
	member := interaction.Member()
	if member == nil {
		return false
	}

	if s.cfg.RoleID != 0 {
		if interaction.GuildID() == nil {
			return false
		}

		needle := snowflake.ID(s.cfg.RoleID)
		for _, roleID := range member.RoleIDs {
			if roleID == needle {
				return true
			}
		}

		return false
	}

	return member.Permissions.Has(discord.PermissionAdministrator)
}
