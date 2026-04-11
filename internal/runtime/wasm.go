package runtime

import (
	"context"
	"errors"

	"github.com/discord-bot-framework/kernel-go/internal/types"
)

type WasmRuntime struct{}

func NewWasmRuntime() *WasmRuntime {
	return &WasmRuntime{}
}

func (r *WasmRuntime) Start(
	ctx context.Context,
	moduleDir string,
	manifest types.Manifest,
) (Instance, error) {
	_ = ctx
	_ = moduleDir
	_ = manifest

	return nil, errors.New("wasm runtime not implemented")
}
