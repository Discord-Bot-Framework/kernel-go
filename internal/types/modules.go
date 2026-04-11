package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/disgoorg/disgo/discord"
)

type Manifest struct {
	Name     string        `json:"name"`
	Version  string        `json:"version"`
	Entry    string        `json:"entry"`
	URL      string        `json:"-"`
	DirName  string        `json:"-"`
	Commands []CommandSpec `json:"commands"`
}

type CommandSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type InitRequest struct {
	Type string `json:"type"`
}

type InitResponse struct {
	Capabilities map[string]any `json:"capabilities,omitempty"`
	Type         string         `json:"type"`
	Commands     []CommandSpec  `json:"commands"`
}

type InteractionRequest struct {
	GuildID       *string        `json:"guild_id,omitempty"`
	Options       map[string]any `json:"options,omitempty"`
	Type          string         `json:"type"`
	InteractionID string         `json:"interaction_id"`
	Token         string         `json:"token"`
	CommandPath   string         `json:"command_path"`
	CommandName   string         `json:"command_name"`
	UserID        string         `json:"user_id"`
	ChannelID     string         `json:"channel_id"`
}

type InteractionResponse struct {
	Type      string `json:"type"`
	Content   string `json:"content,omitempty"`
	Ephemeral bool   `json:"ephemeral,omitempty"`
}

var (
	errInvalidManifest = errors.New("invalid manifest: name, version and entry are required")
	errInvalidModName  = errors.New("invalid module name: contains forbidden characters")
)

func ReadManifest(dir string) (Manifest, error) {
	path := filepath.Join(dir, "kernel.module.json")

	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest file: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}

	m.Name = strings.TrimSpace(m.Name)
	m.Version = strings.TrimSpace(m.Version)

	m.Entry = strings.TrimSpace(m.Entry)

	if m.Name == "" || m.Version == "" || m.Entry == "" {
		return Manifest{}, errInvalidManifest
	}

	if strings.ContainsAny(m.Name, " \t\n/\\") {
		return Manifest{}, errInvalidModName
	}

	return m, nil
}

func (m Manifest) CommandCreates() []discord.ApplicationCommandCreate {
	if len(m.Commands) == 0 {
		return nil
	}

	creates := make([]discord.ApplicationCommandCreate, 0, len(m.Commands))

	for _, c := range m.Commands {
		name := strings.TrimSpace(c.Name)

		desc := strings.TrimSpace(c.Description)

		if name == "" || desc == "" {
			continue
		}

		creates = append(creates, discord.SlashCommandCreate{
			Name:        name,
			Description: desc,
		})
	}

	return creates
}

func (m Manifest) SupportsCommand(name string) bool {
	for _, c := range m.Commands {
		if strings.EqualFold(strings.TrimSpace(c.Name), name) {
			return true
		}
	}

	return false
}
