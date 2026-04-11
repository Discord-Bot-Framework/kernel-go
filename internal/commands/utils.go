package commands

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

func (s *State) authorizeOrReply(e *handler.CommandEvent) (bool, error) {
	if s.Authorize(e.ApplicationCommandInteraction) {
		return true, nil
	}

	return false, e.CreateMessage(
		discord.NewMessageCreate().WithContent("Unauthorized.").WithEphemeral(true),
	)
}

func boolPtr(v bool) *bool {
	return &v
}

func min(a int, b int) int {
	if a < b {
		return a
	}

	return b
}

func writeLines(fn func(w func(string, ...any))) string {
	var b bytes.Buffer

	w := func(format string, args ...any) {
		fmt.Fprintf(&b, format, args...)
		fmt.Fprintln(&b)
	}
	fn(w)

	return trimTrailingNewline(b.String())
}

func trimTrailingNewline(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}

	return s
}

func splitChunks(s string, max int) []string {
	if s == "" {
		return []string{""}
	}

	if max <= 0 {
		return []string{s}
	}

	var out []string
	for len(s) > max {
		out = append(out, s[:max])
		s = s[max:]
	}

	out = append(out, s)

	return out
}

func openDiscordFile(filePath string, name string) (*discord.File, func(), error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}

	closeFn := func() { _ = f.Close() }

	return discord.NewFile(name, "", f), closeFn, nil
}

func recentLogs(dir string, max int) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	type item struct {
		ts   time.Time
		name string
		size int64
	}

	var items []item

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "kernel.log") && !strings.HasSuffix(name, ".log") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		items = append(items, item{name: name, ts: info.ModTime(), size: info.Size()})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].ts.After(items[j].ts) })

	if len(items) > max {
		items = items[:max]
	}

	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, fmt.Sprintf("- `%s` (%d KB)", it.name, it.size/1024))
	}

	return out
}
