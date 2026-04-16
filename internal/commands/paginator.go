package commands

import (
	discordx "github.com/discord-bot-framework/kernel-go/internal/discord"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

func (s *State) handlePaginator(e *handler.ComponentEvent) error {
	sessionID := e.Vars["id"]

	action := e.Vars["action"]
	if sessionID == "" || action == "" {
		return nil
	}

	update, ok, unauthorized := s.paginator.UpdateFromAction(sessionID, action, e.User().ID)
	if unauthorized {
		return e.CreateMessage(discordx.EphemeralError("Unauthorized."))
	}

	if !ok {
		empty := []discord.LayoutComponent{}

		return e.UpdateMessage(discord.MessageUpdate{Components: &empty})
	}

	return e.UpdateMessage(update)
}
