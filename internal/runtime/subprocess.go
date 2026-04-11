package runtime

import (
	"context"
	"log/slog"

	"github.com/discord-bot-framework/kernel-go/internal/types"
)

type SubprocessRuntime struct {
	logger *slog.Logger
}

func NewSubprocessRuntime(logger *slog.Logger) *SubprocessRuntime {
	return &SubprocessRuntime{logger: logger}
}

func (r *SubprocessRuntime) Start(
	ctx context.Context,
	moduleDir string,
	manifest types.Manifest,
) (Instance, error) {
	return StartModule(ctx, r.logger, moduleDir, manifest)
}
