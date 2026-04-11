package utils

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

func NewLogger(baseDir string) (*slog.Logger, func(), error) {
	p, err := NewPaths(baseDir)
	if err != nil {
		return nil, nil, err
	}

	f, err := os.OpenFile(p.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, err
	}

	handler := slog.NewJSONHandler(io.MultiWriter(os.Stderr, f), &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	})
	logger := slog.New(handler).With("app", "kernel", "cwd", filepath.Clean(baseDir))
	closeFn := func() { _ = f.Close() }

	return logger, closeFn, nil
}
