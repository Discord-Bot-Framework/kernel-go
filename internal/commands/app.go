package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	discordx "github.com/discord-bot-framework/kernel-go/internal/discord"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"
)

func (s *State) handleAppExec(
	_ discord.SlashCommandInteractionData,
	e *handler.CommandEvent,
) error {
	ok, err := s.authorizeOrReply(e)
	if !ok {
		return err
	}

	if e.GuildID() == nil {
		return e.CreateMessage(
			discord.NewMessageCreate().
				WithContent("This command is available only in guilds.").
				WithEphemeral(true),
		)
	}

	modal := discord.ModalCreate{
		CustomID: "kernel:app:exec",
		Title:    "Debug Execution",
		Components: []discord.LayoutComponent{
			discord.NewLabel("Code to Run", discord.TextInputComponent{
				CustomID:    "body",
				Style:       discord.TextInputStyleParagraph,
				Required:    true,
				Placeholder: "Write your Go code here!",
			}),
		},
	}

	return e.Modal(modal)
}

func (s *State) handleAppExecModal(e *handler.ModalEvent) error {
	if !s.Authorize(e.ModalSubmitInteraction) {
		return e.CreateMessage(
			discord.NewMessageCreate().WithContent("Unauthorized.").WithEphemeral(true),
		)
	}

	if e.GuildID() == nil {
		return e.CreateMessage(
			discord.NewMessageCreate().
				WithContent("This command is available only in guilds.").
				WithEphemeral(true),
		)
	}

	body, _ := e.Data.OptText("body")
	if strings.TrimSpace(body) == "" {
		return e.CreateMessage(
			discord.NewMessageCreate().WithContent("No code was provided.").WithEphemeral(true),
		)
	}

	out, err := s.modules.EvalGo(e.Ctx, body)
	if err != nil {
		var b bytes.Buffer
		b.WriteString(out)

		if len(out) > 0 && out[len(out)-1] != '\n' {
			fmt.Fprintln(&b)
		}

		fmt.Fprintln(&b, err)
		out = b.String()
	}

	out = redactSecrets(out, s.cfg.Token)

	chunks := splitChunks(out, 1900)
	for i, chunk := range chunks {
		var b bytes.Buffer
		fmt.Fprintln(&b, "```go")
		b.WriteString(chunk)
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "```")

		msg := discord.NewMessageCreate().
			WithContent(trimTrailingNewline(b.String())).
			WithEphemeral(true)
		if i == 0 {
			err := e.CreateMessage(msg)
			if err != nil {
				return err
			}

			continue
		}

		_, _ = e.Client().Rest.CreateFollowupMessage(e.ApplicationID(), e.Token(), msg)
	}

	return nil
}

func (s *State) handleAppInfo(
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

	app, err := e.Client().Rest.GetCurrentApplication()
	if err != nil {
		return e.CreateMessage(discordx.EphemeralError("Failed to retrieve application information."))
	}

	embed := discord.NewEmbed().
		WithTitle("Application Information").
		AddField("ID", fmt.Sprintf("`%s`", app.ID.String()), true).
		AddField("Name", fmt.Sprintf("`%s`", app.Name), true)

	return e.CreateMessage(discord.NewMessageCreate().AddEmbeds(embed).WithEphemeral(true))
}

func (s *State) handleAppSearch(
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

	cmdIDStr, _ := data.OptString("cmd")

	scopeStr, _ := data.OptString("scope")
	if scopeStr == "" {
		scopeStr = "0"
	}

	cmdID, err := snowflake.Parse(cmdIDStr)
	if err != nil {
		return e.CreateMessage(discordx.EphemeralError("Invalid command ID."))
	}

	scopeID, _ := snowflake.Parse(scopeStr)

	app, err := e.Client().Rest.GetCurrentApplication()
	if err != nil {
		return e.CreateMessage(discordx.EphemeralError("Failed to retrieve application information."))
	}

	var cmd discord.ApplicationCommand
	if scopeID != 0 {
		cmd, err = e.Client().Rest.GetGuildCommand(app.ID, scopeID, cmdID)
	} else {
		cmd, err = e.Client().Rest.GetGlobalCommand(app.ID, cmdID)
	}

	if err != nil {
		return e.CreateMessage(discordx.EphemeralError("Failed to fetch command."))
	}

	raw, _ := json.MarshalIndent(cmd, "", "  ")

	return respondChunked(e, "json", string(raw))
}

func (s *State) handleAppScope(
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

	scopeStr, _ := data.OptString("scope")
	if scopeStr == "" {
		scopeStr = "0"
	}

	scopeID, _ := snowflake.Parse(scopeStr)

	app, err := e.Client().Rest.GetCurrentApplication()
	if err != nil {
		return e.CreateMessage(discordx.EphemeralError("Failed to retrieve application information."))
	}

	var cmds []discord.ApplicationCommand
	if scopeID != 0 {
		cmds, err = e.Client().Rest.GetGuildCommands(app.ID, scopeID, false)
	} else {
		cmds, err = e.Client().Rest.GetGlobalCommands(app.ID, false)
	}

	if err != nil {
		return e.CreateMessage(discordx.EphemeralError("Failed to retrieve command list."))
	}

	var b bytes.Buffer
	if scopeID != 0 {
		fmt.Fprintf(&b, "Commands in scope `%s`:", scopeStr)
		fmt.Fprintln(&b)
	} else {
		fmt.Fprintln(&b, "Global Commands:")
	}

	for _, cmd := range cmds {
		fmt.Fprintf(&b, "- `%s` (%s)", cmd.Name(), cmd.ID().String())
		fmt.Fprintln(&b)
	}

	return e.CreateMessage(
		discord.NewMessageCreate().WithContent(trimTrailingNewline(b.String())).WithEphemeral(true),
	)
}

func (s *State) handleAppDelete(
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

	scopeStr, _ := data.OptString("scope")
	cmdIDStr, _ := data.OptString("cmd_id")
	deleteAll, _ := data.OptBool("all")

	if scopeStr == "" {
		scopeStr = "0"
	}

	scopeID, _ := snowflake.Parse(scopeStr)

	app, err := e.Client().Rest.GetCurrentApplication()
	if err != nil {
		return e.CreateMessage(discordx.EphemeralError("Failed to retrieve application information."))
	}

	if deleteAll {
		var cmdCreates []discord.ApplicationCommandCreate
		if scopeID != 0 {
			_, err = e.Client().Rest.SetGuildCommands(app.ID, scopeID, cmdCreates)
		} else {
			_, err = e.Client().Rest.SetGlobalCommands(app.ID, cmdCreates)
		}

		if err != nil {
			return e.CreateMessage(discordx.EphemeralError("Failed to delete commands."))
		}

		return e.CreateMessage(
			discord.NewMessageCreate().WithContent("All commands have been deleted.").WithEphemeral(true),
		)
	}

	if cmdIDStr == "" {
		return e.CreateMessage(discordx.EphemeralError("The cmd_id parameter is required when not deleting all commands."))
	}

	cmdID, err := snowflake.Parse(cmdIDStr)
	if err != nil {
		return e.CreateMessage(discordx.EphemeralError("Invalid command ID."))
	}

	if scopeID != 0 {
		err = e.Client().Rest.DeleteGuildCommand(app.ID, scopeID, cmdID)
	} else {
		err = e.Client().Rest.DeleteGlobalCommand(app.ID, cmdID)
	}

	if err != nil {
		return e.CreateMessage(discordx.EphemeralError("Failed to delete command."))
	}

	return e.CreateMessage(
		discord.NewMessageCreate().WithContent("Command deleted successfully.").WithEphemeral(true),
	)
}

func (s *State) autocompleteAppCommandID(e *handler.AutocompleteEvent) error {
	app, err := e.Client().Rest.GetCurrentApplication()
	if err != nil {
		return e.AutocompleteResult([]discord.AutocompleteChoice{})
	}

	cmds, err := e.Client().Rest.GetGlobalCommands(app.ID, false)
	if err != nil {
		return e.AutocompleteResult([]discord.AutocompleteChoice{})
	}

	choices := make([]discord.AutocompleteChoice, 0, min(25, len(cmds)))
	for _, cmd := range cmds {
		name := fmt.Sprintf("%s (%s)", cmd.Name(), cmd.ID().String())

		choices = append(
			choices,
			discord.AutocompleteChoiceString{Name: name, Value: cmd.ID().String()},
		)
		if len(choices) >= 25 {
			break
		}
	}

	return e.AutocompleteResult(choices)
}

func respondChunked(e *handler.CommandEvent, lang string, content string) error {
	chunks := splitChunks(content, 1900)
	for i, chunk := range chunks {
		var b bytes.Buffer
		fmt.Fprintln(&b, "```"+lang)
		b.WriteString(chunk)
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "```")

		msg := discord.NewMessageCreate().
			WithContent(trimTrailingNewline(b.String())).
			WithEphemeral(true)
		if i == 0 {
			err := e.CreateMessage(msg)
			if err != nil {
				return err
			}

			continue
		}

		_, _ = e.Client().Rest.CreateFollowupMessage(e.ApplicationID(), e.Token(), msg)
	}

	return nil
}

func redactSecrets(s string, token string) string {
	if token == "" {
		return s
	}

	return strings.ReplaceAll(s, token, "[REDACTED TOKEN]")
}
