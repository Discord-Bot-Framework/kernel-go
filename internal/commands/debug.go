package commands

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	discordx "github.com/discord-bot-framework/kernel-go/internal/discord"
	"github.com/discord-bot-framework/kernel-go/internal/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

func (s *State) handleDebugDownload(
	_ discord.SlashCommandInteractionData,
	e *handler.CommandEvent,
) error {
	ok, err := s.authorizeOrReply(e)
	if !ok {
		return err
	}

	if err := e.DeferCreateMessage(true); err != nil {
		return err
	}

	path, cleanup, err := utils.BuildCodeArchive(e.Ctx, s.logger, s.paths.BaseDir)
	if err != nil {
		return e.CreateMessage(
			discord.NewMessageCreate().
				WithContent(fmt.Sprintf("Failed to complete download: %v.", err)).
				WithEphemeral(true),
		)
	}
	defer cleanup()

	file, closeFn, err := openDiscordFile(path, "client_code.tar.zst")
	if err != nil {
		return e.CreateMessage(discordx.EphemeralError("Failed to open archive file."))
	}
	defer closeFn()

	return e.CreateMessage(
		discord.NewMessageCreate().
			WithContent("Code archive attached:").
			AddFiles(file).
			WithEphemeral(true),
	)
}

func (s *State) autocompleteDebugExport(e *handler.AutocompleteEvent) error {
	choices := make([]discord.AutocompleteChoice, 0, 25)
	choices = append(choices, discord.AutocompleteChoiceString{Name: "all", Value: "all"})

	entries, err := os.ReadDir(s.paths.BaseDir)
	if err == nil {
		var names []string

		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}

			if entry.Type().IsRegular() {
				names = append(names, name)
			}
		}

		sort.Slice(
			names,
			func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) },
		)

		for _, name := range names {
			choices = append(choices, discord.AutocompleteChoiceString{Name: name, Value: name})
			if len(choices) >= 25 {
				break
			}
		}
	}

	return e.AutocompleteResult(choices)
}

func (s *State) handleDebugExport(
	data discord.SlashCommandInteractionData,
	e *handler.CommandEvent,
) error {
	ok, err := s.authorizeOrReply(e)
	if !ok {
		return err
	}

	if err := e.DeferCreateMessage(true); err != nil {
		return err
	}

	pathValue, _ := data.OptString("path")
	if pathValue == "" {
		return e.CreateMessage(discordx.EphemeralError("Invalid path format."))
	}

	target, err := s.resolveExportTarget(pathValue)
	if err != nil {
		return e.CreateMessage(discordx.EphemeralError(err.Error()))
	}

	archivePath, cleanup, err := utils.BuildExportArchive(e.Ctx, s.logger, s.paths.BaseDir, target)
	if err != nil {
		return e.CreateMessage(discordx.EphemeralError("Failed to create archive."))
	}
	defer cleanup()

	archiveName := filepath.Base(target) + ".tar.zst"

	file, closeFn, err := openDiscordFile(archivePath, archiveName)
	if err != nil {
		return e.CreateMessage(discordx.EphemeralError("Failed to open archive file."))
	}
	defer closeFn()

	return e.CreateMessage(
		discord.NewMessageCreate().
			WithContent(fmt.Sprintf("Exported `%s`:", pathValue)).
			AddFiles(file).
			WithEphemeral(true),
	)
}

func (s *State) resolveExportTarget(userPath string) (string, error) {
	if userPath == "all" {
		return s.paths.BaseDir, nil
	}

	full, err := utils.ResolveWithinBase(s.paths.BaseDir, userPath)
	if err != nil {
		return "", errors.New("path is outside the allowed directory")
	}

	st, err := os.Stat(full)
	if err != nil {
		return "", errors.New("path does not exist")
	}

	if !st.Mode().IsRegular() && !st.IsDir() {
		return "", errors.New("path is neither a file nor a directory")
	}

	return full, nil
}

func (s *State) handleDebugInfo(
	_ discord.SlashCommandInteractionData,
	e *handler.CommandEvent,
) error {
	ok, err := s.authorizeOrReply(e)
	if !ok {
		return err
	}

	if err := e.DeferCreateMessage(true); err != nil {
		return err
	}

	guildCount := e.Client().Caches.GuildsLen()
	selfUser, _ := e.Client().Caches.SelfUser()

	systemField := writeLines(func(w func(string, ...any)) {
		w("- Go: `%s`", runtime.Version())
		w("- Platform: `%s/%s`", runtime.GOOS, runtime.GOARCH)
		w("- PID: `%d`", os.Getpid())
		w("- CWD: `%s`", s.paths.BaseDir)
		w("- Log: `%s`", filepath.Base(s.paths.LogFile))
	})
	botField := writeLines(func(w func(string, ...any)) {
		w("- User: `%s` (%s)", selfUser.EffectiveName(), selfUser.ID.String())
		w("- Guilds: `%d`", guildCount)
		w("- Modules: `%d`", s.modules.Count())
	})

	embed := discord.NewEmbed().
		WithTitle("System Status").
		WithDescription("Runtime diagnostics and bot information.").
		WithFields(
			discord.EmbedField{Name: "System", Value: systemField, Inline: new(true)},
			discord.EmbedField{Name: "Bot", Value: botField, Inline: new(true)},
		)

	recent := recentLogs(filepath.Dir(s.paths.LogFile), 5)
	if len(recent) > 0 {
		var b bytes.Buffer
		for _, line := range recent {
			fmt.Fprintln(&b, line)
		}

		embed = embed.AddField("Recent Logs", trimTrailingNewline(b.String()), false)
	}

	return e.CreateMessage(discord.NewMessageCreate().AddEmbeds(embed).WithEphemeral(true))
}

func (s *State) handleDebugRestart(
	_ discord.SlashCommandInteractionData,
	e *handler.CommandEvent,
) error {
	ok, err := s.authorizeOrReply(e)
	if !ok {
		return err
	}

	if err := e.DeferCreateMessage(true); err != nil {
		return err
	}

	if err := writeRestartFlag(s.paths.FlagDir, e.User().ID); err != nil {
		s.logger.Error("failed to write restart flag", "err", err)
	}

	s.RequestShutdown("restart requested")

	return e.CreateMessage(
		discord.NewMessageCreate().
			WithContent("Initiated bot restart sequence.").
			WithEphemeral(true),
	)
}
