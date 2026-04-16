package commands

import (
	"context"
	"log/slog"
	"slices"
	"sync"

	"github.com/discord-bot-framework/kernel-go/internal/modules"
	"github.com/discord-bot-framework/kernel-go/internal/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

type State struct {
	logger *slog.Logger

	modules   *modules.Manager
	paginator *utils.PaginatorManager

	shutdownCh chan struct{}
	paths      utils.Paths

	cfg utils.Config

	shutdownOnce sync.Once

	saveShardStates func()
}

func NewState(logger *slog.Logger, cfg utils.Config, baseDir string) *State {
	p, _ := utils.NewPaths(baseDir)

	return &State{
		logger:     logger,
		cfg:        cfg,
		paths:      p,
		modules:    modules.NewManager(logger, cfg, p),
		paginator:  utils.NewPaginatorManager(logger, utils.DefaultPaginatorConfig()),
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

func (s *State) SetShardStateSaver(fn func()) {
	s.saveShardStates = fn
}

func (s *State) SaveShardStates() {
	if s.saveShardStates != nil {
		s.saveShardStates()
	}
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
		return slices.Contains(member.RoleIDs, needle)
	}

	return member.Permissions.Has(discord.PermissionAdministrator)
}
