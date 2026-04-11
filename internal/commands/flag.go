package commands

import (
	"os"
	"path/filepath"
	"time"

	"github.com/disgoorg/snowflake/v2"
)

func writeRestartFlag(flagDir string, executorID snowflake.ID) error {
	err := os.MkdirAll(flagDir, 0o750)
	if err != nil {
		return err
	}

	path := filepath.Join(flagDir, "restart")
	content := writeLines(func(w func(string, ...any)) {
		w(
			"Restart triggered at %s by %s",
			time.Now().UTC().Format(time.RFC3339Nano),
			executorID.String(),
		)
	})

	return os.WriteFile(path, []byte(content), 0o600)
}
