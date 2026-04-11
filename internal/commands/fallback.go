package commands

import (
	"fmt"
	"strings"

	discordx "github.com/discord-bot-framework/kernel-go/internal/discord"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

func (s *State) handleModuleCommandFallback(
	data discord.SlashCommandInteractionData,
	e *handler.CommandEvent,
) error {
	commandPath := data.CommandPath()
	if commandPath == "" || commandPath == "/bot" || strings.HasPrefix(commandPath, "/bot/") {
		return nil
	}

	opts := make(map[string]any, len(data.Options))
	for name, opt := range data.Options {
		switch opt.Type {
		case discord.ApplicationCommandOptionTypeString:
			opts[name] = opt.String()
		case discord.ApplicationCommandOptionTypeInt:
			opts[name] = opt.Int()
		case discord.ApplicationCommandOptionTypeBool:
			opts[name] = opt.Bool()
		case discord.ApplicationCommandOptionTypeFloat:
			opts[name] = opt.Float()
		default:
			opts[name] = string(opt.Value)
		}
	}

	resp, handled, err := s.modules.DispatchInteraction(
		e.Ctx,
		commandPath,
		e.ID(),
		e.Token(),
		e.User().ID,
		e.GuildID(),
		e.Channel().ID(),
		opts,
	)
	if err != nil {
		return e.CreateMessage(
			discordx.EphemeralError(fmt.Sprintf("Module execution failed: %v", err)),
		)
	}

	if !handled {
		return e.CreateMessage(
			discord.NewMessageCreate().WithContent("Command not found.").WithEphemeral(true),
		)
	}

	msg := discord.NewMessageCreate().WithContent(resp.Content).WithEphemeral(resp.Ephemeral)

	return e.CreateMessage(msg)
}
