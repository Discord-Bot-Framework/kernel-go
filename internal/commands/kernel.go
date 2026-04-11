package commands

import (
	"fmt"

	discordx "github.com/discord-bot-framework/kernel-go/internal/discord"
	"github.com/discord-bot-framework/kernel-go/internal/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

func (s *State) handleKernelInfo(
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

	info, err := utils.GetKernelInfo(s.paths.BaseDir)
	if err != nil {
		return e.CreateMessage(
			discordx.EphemeralError("Failed to retrieve kernel repository information."),
		)
	}

	embed := discord.NewEmbed().
		WithTitle("Kernel Info").
		WithDescription("Kernel repository status.").
		AddField("URL", fmt.Sprintf("`%s`", info.URL), false).
		AddField("Current Commit", fmt.Sprintf("`%s`", info.LocalCommitID), true).
		AddField("Target Commit", fmt.Sprintf("`%s`", info.RemoteCommitID), true).
		AddField("Uncommitted Changes", fmt.Sprintf("`%d`", info.UncommittedChanges), true)
	if info.LocalCommitTimeUTC != "" {
		embed = embed.AddField(
			"Current Time (UTC)",
			fmt.Sprintf("`%s`", info.LocalCommitTimeUTC),
			true,
		)
	}

	if info.RemoteCommitTimeUTC != "" {
		embed = embed.AddField(
			"Target Time (UTC)",
			fmt.Sprintf("`%s`", info.RemoteCommitTimeUTC),
			true,
		)
	}

	return e.CreateMessage(discord.NewMessageCreate().AddEmbeds(embed).WithEphemeral(true))
}

func (s *State) handleKernelUpdate(
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

	info, err := utils.GetKernelInfo(s.paths.BaseDir)
	if err != nil {
		return e.CreateMessage(
			discordx.EphemeralError("Failed to retrieve kernel repository information."),
		)
	}

	if info.UpToDate && info.UncommittedChanges == 0 {
		return e.CreateMessage(
			discord.NewMessageCreate().
				WithContent("Kernel already up-to-date (no local modifications).").
				WithEphemeral(true),
		)
	}

	if err := utils.PullToRemote(s.logger, s.paths.BaseDir); err != nil {
		return e.CreateMessage(
			discordx.EphemeralError(fmt.Sprintf("Failed to update kernel: %v.", err)),
		)
	}

	_ = writeRestartFlag(s.paths.FlagDir, e.User().ID)
	s.RequestShutdown("kernel updated")

	return e.CreateMessage(
		discord.NewMessageCreate().
			WithContent("Kernel update complete. Signaling restart now.").
			WithEphemeral(true),
	)
}
