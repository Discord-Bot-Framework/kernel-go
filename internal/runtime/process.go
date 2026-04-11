package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/discord-bot-framework/kernel-go/internal/types"
)

type process struct {
	stdin  io.WriteCloser
	logger *slog.Logger
	cmd    *exec.Cmd
	stdout *bufio.Reader

	manifest types.Manifest

	mu sync.Mutex
}

func StartModule(
	ctx context.Context,
	logger *slog.Logger,
	moduleDir string,
	manifest types.Manifest,
) (*process, error) {
	exePath, err := prepareEntry(ctx, logger, moduleDir, manifest)
	if err != nil {
		return nil, fmt.Errorf("prepare entry: %w", err)
	}

	cmd := exec.CommandContext(ctx, exePath)
	cmd.Dir = moduleDir
	cmd.Env = cleanEnv(os.Environ())

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("get stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("get stdout pipe: %w", err)
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdoutPipe.Close()

		return nil, fmt.Errorf("start command: %w", err)
	}

	p := &process{
		logger:   logger,
		manifest: manifest,
		cmd:      cmd,
		stdin:    stdin,
		stdout:   bufio.NewReader(stdoutPipe),
	}

	initCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := p.init(initCtx); err != nil {
		p.Stop(context.Background())

		return nil, fmt.Errorf("init process: %w", err)
	}

	return p, nil
}

func (p *process) Running() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}

	return p.cmd.ProcessState == nil || !p.cmd.ProcessState.Exited()
}

func (p *process) Stop(ctx context.Context) {
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}

	_ = p.stdin.Close()
	done := make(chan struct{})

	go func() {
		_ = p.cmd.Wait()

		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		_ = p.cmd.Process.Kill()

		<-done
	}
}

var errInvalidInitResp = errors.New("invalid init response")

func (p *process) init(ctx context.Context) (types.InitResponse, error) {
	req := types.InitRequest{Type: "init"}
	if err := p.writeJSON(req); err != nil {
		return types.InitResponse{}, fmt.Errorf("write init request: %w", err)
	}

	line, err := p.readLine(ctx)
	if err != nil {
		return types.InitResponse{}, fmt.Errorf("read init response: %w", err)
	}

	var resp types.InitResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return types.InitResponse{}, fmt.Errorf("unmarshal init response: %w", err)
	}

	if resp.Type != "init" {
		return types.InitResponse{}, errInvalidInitResp
	}

	return resp, nil
}

var errInvalidInteractionResp = errors.New("invalid interaction response")

func (p *process) Call(
	ctx context.Context,
	req types.InteractionRequest,
) (types.InteractionResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	req.Type = "interaction"
	if err := p.writeJSON(req); err != nil {
		return types.InteractionResponse{}, fmt.Errorf("write interaction request: %w", err)
	}

	line, err := p.readLine(ctx)
	if err != nil {
		return types.InteractionResponse{}, fmt.Errorf("read interaction response: %w", err)
	}

	var resp types.InteractionResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return types.InteractionResponse{}, fmt.Errorf("unmarshal interaction response: %w", err)
	}

	if resp.Type != "interaction" {
		return types.InteractionResponse{}, errInvalidInteractionResp
	}

	return resp, nil
}

func (p *process) writeJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	data = append(data, '\n')
	if _, err = p.stdin.Write(data); err != nil {
		return fmt.Errorf("write to stdin: %w", err)
	}

	return nil
}

func (p *process) readLine(ctx context.Context) (string, error) {
	type res struct {
		err  error
		line string
	}

	ch := make(chan res, 1)

	go func() {
		line, err := p.stdout.ReadString('\n')
		ch <- res{line: strings.TrimSpace(line), err: err}
	}()

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("context canceled: %w", ctx.Err())
	case r := <-ch:
		if r.err != nil {
			return "", fmt.Errorf("read from stdout: %w", r.err)
		}

		return r.line, nil
	}
}

var errInvalidEntry = errors.New("invalid entry")

func prepareEntry(
	ctx context.Context,
	logger *slog.Logger,
	moduleDir string,
	manifest types.Manifest,
) (string, error) {
	entry := filepath.Clean(manifest.Entry)
	if entry == "" || strings.HasPrefix(entry, "..") || filepath.IsAbs(entry) {
		return "", errInvalidEntry
	}

	entryPath := filepath.Join(moduleDir, entry)
	if st, err := os.Stat(entryPath); err == nil && st.Mode().IsRegular() && st.Mode()&0o111 != 0 {
		return entryPath, nil
	}

	outDir := filepath.Join(moduleDir, "bin")

	err := os.MkdirAll(outDir, 0o750)
	if err != nil {
		return "", fmt.Errorf("create bin directory: %w", err)
	}

	outPath := filepath.Join(outDir, manifest.Name+"_module")

	buildCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		buildCtx,
		"go",
		"build",
		"-trimpath",
		"-ldflags",
		"-s -w",
		"-o",
		outPath,
		entry,
	)
	cmd.Dir = moduleDir
	cmd.Env = cleanEnv(os.Environ())
	cmd.Stdout = os.Stdout

	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		logger.Error("module build failed", "module", manifest.Name, "err", err)

		return "", fmt.Errorf("build module: %w", err)
	}

	return outPath, nil
}

func cleanEnv(env []string) []string {
	out := make([]string, 0, len(env))

	for _, kv := range env {
		if strings.HasPrefix(kv, "TOKEN=") {
			continue
		}

		out = append(out, kv)
	}

	return out
}
