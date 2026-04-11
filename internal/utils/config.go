package utils

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Token   string
	GuildID uint64
	RoleID  uint64
}

func LoadConfig(baseDir string) (Config, error) {
	_ = godotenv.Load(filepath.Join(baseDir, ".env"))

	cfg := Config{
		Token:   strings.TrimSpace(os.Getenv("TOKEN")),
		GuildID: envUint64("GUILD_ID"),
		RoleID:  envUint64("ROLE_ID"),
	}

	_ = os.Unsetenv("TOKEN")
	_ = os.Remove(filepath.Join(baseDir, ".env"))

	if cfg.Token == "" {
		return Config{}, errors.New("TOKEN is required")
	}

	return cfg, nil
}

func envUint64(name string) uint64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0
	}

	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0
	}

	return value
}
