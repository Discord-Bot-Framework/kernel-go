package modules

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/discord-bot-framework/kernel-go/internal/runtime"
	"github.com/discord-bot-framework/kernel-go/internal/types"
	"github.com/discord-bot-framework/kernel-go/internal/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

type Manager struct {
	rt      runtime.Runtime
	logger  *slog.Logger
	modules map[string]*Loaded

	eval  *Evaluator
	paths utils.Paths

	cfg utils.Config

	mu sync.RWMutex
}

type Loaded struct {
	Proc     runtime.Instance
	Manifest types.Manifest
}

type ModuleListItem struct {
	Name    string
	Version string
}

type ModuleInfo struct {
	Name    string
	Version string
	DirName string
	URL     string
	Running bool
}

func NewManager(logger *slog.Logger, cfg utils.Config, p utils.Paths) *Manager {
	return &Manager{
		logger:  logger,
		cfg:     cfg,
		paths:   p,
		modules: make(map[string]*Loaded),
		eval:    NewEvaluator(logger, cfg.Token),
		rt:      runtime.NewSubprocessRuntime(logger),
	}
}

func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.modules)
}

func (m *Manager) KnownModuleNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]string, 0, len(m.modules))
	for name := range m.modules {
		out = append(out, name)
	}

	sortStringsCaseFold(out)

	return out
}

func (m *Manager) ListModules() []ModuleListItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]ModuleListItem, 0, len(m.modules))
	for _, l := range m.modules {
		out = append(out, ModuleListItem{Name: l.Manifest.Name, Version: l.Manifest.Version})
	}

	sort.Slice(
		out,
		func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) },
	)

	return out
}

func (m *Manager) ModuleInfo(name string) (ModuleInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	l, ok := m.modules[name]
	if !ok {
		return ModuleInfo{}, false
	}

	running := l.Proc != nil && l.Proc.Running()

	return ModuleInfo{
		Name:    l.Manifest.Name,
		Version: l.Manifest.Version,
		DirName: l.Manifest.DirName,
		URL:     l.Manifest.URL,
		Running: running,
	}, true
}

func (m *Manager) Commands() []discord.ApplicationCommandCreate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var creates []discord.ApplicationCommandCreate
	for _, l := range m.modules {
		creates = append(creates, l.Manifest.CommandCreates()...)
	}

	return creates
}

func (m *Manager) Load(ctx context.Context, gitURL string) (string, error) {
	parsed, err := validateGitURL(gitURL)
	if err != nil {
		return "", err
	}

	dirName := deriveDirName(parsed)
	if dirName == "" {
		return "", fmt.Errorf("failed to derive module name from URL: %s", gitURL)
	}

	dir := filepath.Join(m.paths.ExtensionsDir, dirName)

	if err := os.MkdirAll(m.paths.ExtensionsDir, 0o750); err != nil {
		return "", fmt.Errorf("create extensions directory: %w", err)
	}

	if _, err := os.Stat(dir); err == nil {
		return "", fmt.Errorf("module already exists: %s", dirName)
	}

	if err := Clone(ctx, parsed.String(), dir); err != nil {
		_ = os.RemoveAll(dir)

		return "", fmt.Errorf("clone module: %w", err)
	}

	manifest, err := types.ReadManifest(dir)
	if err != nil {
		_ = os.RemoveAll(dir)

		return "", fmt.Errorf("read manifest: %w", err)
	}

	manifest.URL = parsed.String()
	manifest.DirName = dirName

	proc, err := m.rt.Start(ctx, dir, manifest)
	if err != nil {
		_ = os.RemoveAll(dir)

		return "", fmt.Errorf("start module: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.modules[manifest.Name]; exists {
		proc.Stop(context.Background())

		_ = os.RemoveAll(dir)

		return "", fmt.Errorf("module name already loaded: %s", manifest.Name)
	}

	m.modules[manifest.Name] = &Loaded{Manifest: manifest, Proc: proc}
	m.logger.Info("module loaded", "module", manifest.Name, "dir", dirName, "url", manifest.URL)

	return manifest.Name, nil
}

func (m *Manager) Unload(ctx context.Context, name string, remove bool) error {
	m.mu.Lock()

	l, ok := m.modules[name]
	if ok {
		delete(m.modules, name)
	}
	m.mu.Unlock()

	if !ok {
		return errors.New("module not loaded")
	}

	if l.Proc != nil {
		l.Proc.Stop(ctx)
	}

	if remove {
		_ = os.RemoveAll(filepath.Join(m.paths.ExtensionsDir, l.Manifest.DirName))
	}

	m.logger.Info("module unloaded", "module", name, "remove", remove)

	return nil
}

func (m *Manager) Update(ctx context.Context, name string) error {
	m.mu.RLock()
	l, ok := m.modules[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("module not loaded: %s", name)
	}

	dir := filepath.Join(m.paths.ExtensionsDir, l.Manifest.DirName)
	if err := Pull(ctx, dir); err != nil {
		return fmt.Errorf("pull module: %w", err)
	}

	manifest, err := types.ReadManifest(dir)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	manifest.URL = l.Manifest.URL
	manifest.DirName = l.Manifest.DirName

	proc, err := m.rt.Start(ctx, dir, manifest)
	if err != nil {
		return fmt.Errorf("start module: %w", err)
	}

	m.mu.Lock()
	old := m.modules[name]
	m.modules[name] = &Loaded{Manifest: manifest, Proc: proc}
	m.mu.Unlock()

	if old != nil && old.Proc != nil {
		old.Proc.Stop(context.Background())
	}

	m.logger.Info("module updated", "module", name)

	return nil
}

func (m *Manager) StopAll(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, l := range m.modules {
		if l.Proc != nil {
			l.Proc.Stop(ctx)
		}

		m.logger.Info("module stopped", "module", name)
	}
}

func (m *Manager) DispatchInteraction(
	ctx context.Context,
	commandPath string,
	interactionID snowflake.ID,
	token string,
	userID snowflake.ID,
	guildID *snowflake.ID,
	channelID snowflake.ID,
	options map[string]any,
) (types.InteractionResponse, bool, error) {
	trimmed := strings.TrimPrefix(commandPath, "/")
	cmdName := strings.SplitN(trimmed, "/", 2)[0]

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, l := range m.modules {
		if l.Proc == nil {
			continue
		}

		if !l.Manifest.SupportsCommand(cmdName) {
			continue
		}

		req := types.InteractionRequest{
			InteractionID: interactionID.String(),
			Token:         token,
			CommandPath:   commandPath,
			CommandName:   cmdName,
			UserID:        userID.String(),
			GuildID:       snowflakePtrString(guildID),
			ChannelID:     channelID.String(),
			Options:       options,
		}
		resp, err := l.Proc.Call(ctx, req)

		return resp, true, err
	}

	return types.InteractionResponse{}, false, nil
}

func (m *Manager) EvalGo(ctx context.Context, code string) (string, error) {
	return m.eval.Eval(ctx, code)
}

func snowflakePtrString(id *snowflake.ID) *string {
	if id == nil {
		return nil
	}

	v := id.String()

	return &v
}

func validateGitURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid git URL: %w", err)
	}

	if parsed.Scheme != "https" || parsed.Host == "" || !strings.Contains(parsed.Host, ".") {
		return nil, errors.New("invalid git URL: must be HTTPS with valid host")
	}

	if !strings.HasSuffix(parsed.Path, ".git") {
		return nil, errors.New("invalid git URL: must end with .git")
	}

	return parsed, nil
}

func deriveDirName(u *url.URL) string {
	hostParts := strings.Split(u.Hostname(), ".")

	var relevant []string

	for _, p := range hostParts {
		if p == "www" || p == "com" {
			continue
		}

		relevant = append(relevant, p)
	}

	relevantNetloc := strings.Join(relevant, ".")
	if relevantNetloc == "" {
		return ""
	}

	pathPart := strings.TrimPrefix(strings.TrimSuffix(u.Path, ".git"), "/")
	if pathPart == "" {
		return ""
	}

	return translateName(relevantNetloc) + "__" + translateName(pathPart)
}

func translateName(s string) string {
	var b strings.Builder

	b.Grow(len(s) * 3)

	for _, r := range s {
		switch r {
		case '_':
			b.WriteString("_u_")
		case '/':
			b.WriteString("_s_")
		case '.':
			b.WriteString("_d_")
		case '-':
			b.WriteString("_h_")
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

func sortStringsCaseFold(v []string) {
	sort.Slice(v, func(i, j int) bool {
		return strings.ToLower(v[i]) < strings.ToLower(v[j])
	})
}
