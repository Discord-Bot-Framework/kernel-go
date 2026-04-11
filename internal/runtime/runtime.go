package runtime

import (
	"context"

	"github.com/discord-bot-framework/kernel-go/internal/types"
)

type Instance interface {
	Running() bool
	Stop(ctx context.Context)
	Call(ctx context.Context, req types.InteractionRequest) (types.InteractionResponse, error)
}

type Runtime interface {
	Start(ctx context.Context, moduleDir string, manifest types.Manifest) (Instance, error)
}
