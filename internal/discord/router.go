package discord

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

type Router struct {
	mux    *handler.Mux
	logger *slog.Logger
}

func NewRouter(logger *slog.Logger, cfg any) *Router {
	_ = cfg
	mux := handler.New()
	mux.Use(recoverMiddleware(logger), timeoutMiddleware(10*time.Second))

	return &Router{mux: mux, logger: logger}
}

func (r *Router) OnEvent(event bot.Event) {
	r.mux.OnEvent(event)
}

func (r *Router) Slash(pattern string, h handler.SlashCommandHandler) {
	r.mux.SlashCommand(pattern, h)
}

func (r *Router) Autocomplete(pattern string, h handler.AutocompleteHandler) {
	r.mux.Autocomplete(pattern, h)
}

func (r *Router) Modal(pattern string, h handler.ModalHandler) {
	r.mux.Modal(pattern, h)
}

func (r *Router) Component(pattern string, h handler.ComponentHandler) {
	r.mux.Component(pattern, h)
}

func recoverMiddleware(logger *slog.Logger) handler.Middleware {
	return func(next handler.Handler) handler.Handler {
		return func(e *handler.InteractionEvent) error {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error(
						"panic in interaction handler",
						"panic",
						rec,
						"stack",
						string(debug.Stack()),
					)
				}
			}()

			return next(e)
		}
	}
}

func timeoutMiddleware(d time.Duration) handler.Middleware {
	return func(next handler.Handler) handler.Handler {
		return func(e *handler.InteractionEvent) error {
			ctx, cancel := context.WithTimeout(e.Ctx, d)
			defer cancel()

			e.Ctx = ctx

			return next(e)
		}
	}
}

var ErrUnauthorized = errors.New("unauthorized")

func EphemeralError(content string) discord.MessageCreate {
	return discord.NewMessageCreate().
		WithContent(content).
		WithEphemeral(true)
}
