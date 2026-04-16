package utils

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

const PaginatorCustomIDPrefix = "/paginator"

type ExpireMode int

const (
	ExpireModeAfterCreation ExpireMode = iota
	ExpireModeAfterLastUsage
)

type PaginatorConfig struct {
	NoPermissionMessage string
	CleanupInterval     time.Duration
	ExpireTime          time.Duration
	EmbedColor          int
}

func DefaultPaginatorConfig() PaginatorConfig {
	return PaginatorConfig{
		NoPermissionMessage: "You can't interact with this paginator because it's not yours.",
		CleanupInterval:     30 * time.Second,
		ExpireTime:          5 * time.Minute,
		EmbedColor:          0x4c50c1,
	}
}

type PaginatorPage struct {
	Title       string
	Description string
	Fields      []discord.EmbedField
}

type PaginatorSession struct {
	ID         string
	Creator    snowflake.ID
	ExpireMode ExpireMode
	Pages      []PaginatorPage

	current  int
	lastUsed time.Time
}

type PaginatorManager struct {
	cfg PaginatorConfig

	mu       sync.Mutex
	sessions map[string]*PaginatorSession
}

func NewPaginatorManager(logger *slog.Logger, cfg PaginatorConfig) *PaginatorManager {
	m := &PaginatorManager{
		cfg:      cfg,
		sessions: make(map[string]*PaginatorSession),
	}
	_ = logger

	go m.cleanupLoop()

	return m
}

func (m *PaginatorManager) cleanupLoop() {
	if m.cfg.CleanupInterval <= 0 {
		return
	}

	ticker := time.NewTicker(m.cfg.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup()
	}
}

func (m *PaginatorManager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cfg.ExpireTime <= 0 {
		return
	}

	now := time.Now()
	for id, session := range m.sessions {
		if now.Sub(session.lastUsed) > m.cfg.ExpireTime {
			delete(m.sessions, id)
		}
	}
}

func (m *PaginatorManager) NewSession(
	creator snowflake.ID,
	expireMode ExpireMode,
	pages []PaginatorPage,
) (*PaginatorSession, error) {
	if len(pages) == 0 {
		return nil, errors.New("no pages")
	}

	id, err := randomID(20)
	if err != nil {
		return nil, err
	}

	s := &PaginatorSession{
		ID:         id,
		Creator:    creator,
		ExpireMode: expireMode,
		Pages:      pages,
		current:    0,
		lastUsed:   time.Now(),
	}

	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()

	return s, nil
}

func (m *PaginatorManager) MessageCreate(
	session *PaginatorSession,
	ephemeral bool,
) discord.MessageCreate {
	msg := discord.NewMessageCreate().
		AddEmbeds(m.embedFor(session)).
		WithComponents(m.componentsFor(session))
	if ephemeral {
		msg = msg.WithEphemeral(true)
	}

	return msg
}

func (m *PaginatorManager) MessageUpdate(session *PaginatorSession) discord.MessageUpdate {
	embeds := []discord.Embed{m.embedFor(session)}
	components := []discord.LayoutComponent{m.componentsFor(session)}

	return discord.MessageUpdate{Embeds: &embeds, Components: &components}
}

func (m *PaginatorManager) UpdateFromAction(
	sessionID, action string,
	actor snowflake.ID,
) (update discord.MessageUpdate, ok bool, unauthorized bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return discord.MessageUpdate{}, false, false
	}

	if session.Creator != 0 && actor != session.Creator {
		return discord.MessageUpdate{}, false, true
	}

	switch action {
	case "first":
		session.current = 0
	case "back":
		if session.current > 0 {
			session.current--
		}
	case "next":
		if session.current < len(session.Pages)-1 {
			session.current++
		}
	case "last":
		session.current = len(session.Pages) - 1
	case "stop":
		delete(m.sessions, sessionID)

		empty := []discord.LayoutComponent{}

		return discord.MessageUpdate{Components: &empty}, true, false
	default:
		return discord.MessageUpdate{}, false, false
	}

	if session.ExpireMode == ExpireModeAfterLastUsage {
		session.lastUsed = time.Now()
	}

	return m.MessageUpdate(session), true, false
}

func (m *PaginatorManager) embedFor(session *PaginatorSession) discord.Embed {
	page := session.Pages[session.current]

	footer := fmt.Sprintf("Page: %d/%d", session.current+1, len(session.Pages))
	embed := discord.NewEmbed().
		WithTitle(page.Title).
		WithDescription(page.Description).
		WithColor(m.cfg.EmbedColor).
		WithFooterText(footer)

	if len(page.Fields) > 0 {
		embed = embed.WithFields(page.Fields...)
	}

	return embed
}

func (m *PaginatorManager) componentsFor(session *PaginatorSession) discord.LayoutComponent {
	return discord.ActionRowComponent{
		Components: []discord.InteractiveComponent{
			discord.ButtonComponent{
				Style:    discord.ButtonStylePrimary,
				Emoji:    &discord.ComponentEmoji{Name: "⏮"},
				CustomID: formatPaginatorCustomID(session.ID, "first"),
				Disabled: session.current == 0,
			},
			discord.ButtonComponent{
				Style:    discord.ButtonStylePrimary,
				Emoji:    &discord.ComponentEmoji{Name: "◀"},
				CustomID: formatPaginatorCustomID(session.ID, "back"),
				Disabled: session.current == 0,
			},
			discord.ButtonComponent{
				Style:    discord.ButtonStyleDanger,
				Emoji:    &discord.ComponentEmoji{Name: "🗑"},
				CustomID: formatPaginatorCustomID(session.ID, "stop"),
			},
			discord.ButtonComponent{
				Style:    discord.ButtonStylePrimary,
				Emoji:    &discord.ComponentEmoji{Name: "▶"},
				CustomID: formatPaginatorCustomID(session.ID, "next"),
				Disabled: session.current == len(session.Pages)-1,
			},
			discord.ButtonComponent{
				Style:    discord.ButtonStylePrimary,
				Emoji:    &discord.ComponentEmoji{Name: "⏩"},
				CustomID: formatPaginatorCustomID(session.ID, "last"),
				Disabled: session.current == len(session.Pages)-1,
			},
		},
	}
}

func formatPaginatorCustomID(sessionID, action string) string {
	var b strings.Builder
	b.Grow(len(PaginatorCustomIDPrefix) + 1 + len(sessionID) + 1 + len(action))
	b.WriteString(PaginatorCustomIDPrefix)
	b.WriteByte('/')
	b.WriteString(sessionID)
	b.WriteByte('/')
	b.WriteString(action)

	return b.String()
}

func ParsePaginatorCustomID(customID string) (sessionID, action string, ok bool) {
	if !strings.HasPrefix(customID, PaginatorCustomIDPrefix+"/") {
		return "", "", false
	}

	rest := strings.TrimPrefix(customID, PaginatorCustomIDPrefix+"/")

	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	if parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	return parts[0], parts[1], true
}

func randomID(n int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	if n <= 0 {
		return "", errors.New("invalid id length")
	}

	var b strings.Builder
	b.Grow(n)

	max := big.NewInt(int64(len(alphabet)))
	for range n {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}

		b.WriteByte(alphabet[v.Int64()])
	}

	return b.String(), nil
}
