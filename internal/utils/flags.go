package utils

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func WriteReadyFlag(flagDir string, pid int) error {
	err := os.MkdirAll(flagDir, 0o750)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "Ready at %s (UTC)", time.Now().UTC().Format(time.RFC3339Nano))
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "PID: %d", pid)
	fmt.Fprintln(&b)

	path := filepath.Join(flagDir, "ready")

	return os.WriteFile(path, bytes.TrimRight(b.Bytes(), "\n"), 0o600)
}
