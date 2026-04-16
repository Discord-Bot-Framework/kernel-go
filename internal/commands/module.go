package commands

import (
	"bytes"
	"fmt"

	discordx "github.com/discord-bot-framework/kernel-go/internal/discord"
	"github.com/discord-bot-framework/kernel-go/internal/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

func (s *State) autocompleteModuleName(e *handler.AutocompleteEvent) error {
	names := s.modules.KnownModuleNames()

	choices := make([]discord.AutocompleteChoice, 0, min(25, len(names)))
	for _, name := range names {
		choices = append(choices, discord.AutocompleteChoiceString{Name: name, Value: name})
		if len(choices) >= 25 {
			break
		}
	}

	return e.AutocompleteResult(choices)
}

func (s *State) handleModuleList(
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

	items := s.modules.ListModules()
	if len(items) == 0 {
		return e.CreateMessage(
			discord.NewMessageCreate().WithContent("No modules loaded.").WithEphemeral(true),
		)
	}

	const perPage = 10
	if len(items) > perPage {
		pages := make([]utils.PaginatorPage, 0, (len(items)+perPage-1)/perPage)
		for i := 0; i < len(items); i += perPage {
			end := min(i+perPage, len(items))

			var b bytes.Buffer
			for _, it := range items[i:end] {
				fmt.Fprintf(&b, "- `%s` (%s)", it.Name, it.Version)
				fmt.Fprintln(&b)
			}

			pages = append(pages, utils.PaginatorPage{
				Title:       "Loaded Modules",
				Description: trimTrailingNewline(b.String()),
			})
		}

		session, err := s.paginator.NewSession(e.User().ID, utils.ExpireModeAfterLastUsage, pages)
		if err != nil {
			return e.CreateMessage(discordx.EphemeralError("Failed to create paginator."))
		}

		return e.CreateMessage(s.paginator.MessageCreate(session, true))
	}

	var b bytes.Buffer
	fmt.Fprintln(&b, "Loaded modules:")

	for _, it := range items {
		fmt.Fprintf(&b, "- `%s` (%s)", it.Name, it.Version)
		fmt.Fprintln(&b)
	}

	return e.CreateMessage(
		discord.NewMessageCreate().WithContent(trimTrailingNewline(b.String())).WithEphemeral(true),
	)
}

func (s *State) handleModuleInfo(
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

	name, _ := data.OptString("module")
	if name == "" {
		return e.CreateMessage(discordx.EphemeralError("Module name is required."))
	}

	info, ok := s.modules.ModuleInfo(name)
	if !ok {
		return e.CreateMessage(discordx.EphemeralError("Module not found."))
	}

	embed := discord.NewEmbed().
		WithTitle("Module Info").
		WithFields(
			discord.EmbedField{
				Name:   "Name",
				Value:  fmt.Sprintf("`%s`", info.Name),
				Inline: new(true),
			},
			discord.EmbedField{
				Name:   "Version",
				Value:  fmt.Sprintf("`%s`", info.Version),
				Inline: new(true),
			},
			discord.EmbedField{
				Name:   "Dir",
				Value:  fmt.Sprintf("`%s`", info.DirName),
				Inline: new(false),
			},
			discord.EmbedField{
				Name:   "Running",
				Value:  fmt.Sprintf("`%t`", info.Running),
				Inline: new(true),
			},
			discord.EmbedField{
				Name:   "URL",
				Value:  fmt.Sprintf("`%s`", info.URL),
				Inline: new(false),
			},
		)

	return e.CreateMessage(discord.NewMessageCreate().AddEmbeds(embed).WithEphemeral(true))
}

func (s *State) handleModuleLoad(
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

	url, _ := data.OptString("url")
	if url == "" {
		return e.CreateMessage(discordx.EphemeralError("Git repo URL is required."))
	}

	name, err := s.modules.Load(e.Ctx, url)
	if err != nil {
		return e.CreateMessage(
			discordx.EphemeralError(fmt.Sprintf("Failed to load module: %v", err)),
		)
	}

	if err := SyncCommands(e.Ctx, s.logger, s.cfg, e.Client(), s); err != nil {
		s.logger.Error("failed to resync commands after module load", "err", err)
	}

	return e.CreateMessage(
		discord.NewMessageCreate().
			WithContent(fmt.Sprintf("Loaded module `%s`.", name)).
			WithEphemeral(true),
	)
}

func (s *State) handleModuleUnload(
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

	name, _ := data.OptString("module")
	if name == "" {
		return e.CreateMessage(discordx.EphemeralError("Module name is required."))
	}

	if err := s.modules.Unload(e.Ctx, name, true); err != nil {
		return e.CreateMessage(
			discordx.EphemeralError(fmt.Sprintf("Failed to unload module: %v", err)),
		)
	}

	if err := SyncCommands(e.Ctx, s.logger, s.cfg, e.Client(), s); err != nil {
		s.logger.Error("failed to resync commands after module unload", "err", err)
	}

	return e.CreateMessage(
		discord.NewMessageCreate().
			WithContent(fmt.Sprintf("Unloaded module `%s`.", name)).
			WithEphemeral(true),
	)
}

func (s *State) handleModuleUpdate(
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

	name, _ := data.OptString("module")
	if name == "" {
		return e.CreateMessage(discordx.EphemeralError("Module name is required."))
	}

	if err := s.modules.Update(e.Ctx, name); err != nil {
		return e.CreateMessage(
			discordx.EphemeralError(fmt.Sprintf("Failed to update module: %v", err)),
		)
	}

	if err := SyncCommands(e.Ctx, s.logger, s.cfg, e.Client(), s); err != nil {
		s.logger.Error("failed to resync commands after module update", "err", err)
	}

	return e.CreateMessage(
		discord.NewMessageCreate().
			WithContent(fmt.Sprintf("Updated module `%s`.", name)).
			WithEphemeral(true),
	)
}
