package utils

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
)

var (
	excludedNames = map[string]struct{}{
		".git":        {},
		"venv":        {},
		".venv":       {},
		"__pycache__": {},
		".env":        {},
		".bak":        {},
		"flag":        {},
	}
	excludedPatterns = []string{"*.pyc", "*.log"}
)

type CleanupFunc func()

func BuildCodeArchive(
	ctx context.Context,
	logger *slog.Logger,
	baseDir string,
) (string, CleanupFunc, error) {
	_ = ctx

	f, err := os.CreateTemp("", "Discord-Bot-Framework_*.tar.zst")
	if err != nil {
		return "", nil, err
	}

	tmp := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)

		return "", nil, err
	}

	if err := buildTarZst(logger, tmp, baseDir, ".", ".", shouldExclude); err != nil {
		_ = os.Remove(tmp)

		return "", nil, err
	}

	return tmp, func() { _ = os.Remove(tmp) }, nil
}

func BuildExportArchive(
	ctx context.Context,
	logger *slog.Logger,
	baseDir string,
	target string,
) (string, CleanupFunc, error) {
	_ = ctx

	if target == "" {
		return "", nil, errors.New("target is empty")
	}

	baseName := filepath.Base(target)

	f, err := os.CreateTemp("", "export_*.tar.zst")
	if err != nil {
		return "", nil, err
	}

	tmp := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)

		return "", nil, err
	}

	if err := buildTarZst(logger, tmp, baseDir, target, baseName, nil); err != nil {
		_ = os.Remove(tmp)

		return "", nil, err
	}

	return tmp, func() { _ = os.Remove(tmp) }, nil
}

type excludeFunc func(tarName string) bool

func shouldExclude(tarName string) bool {
	p := path.Clean(strings.TrimPrefix(tarName, "/"))
	if p == "." || p == "" {
		return false
	}

	for part := range strings.SplitSeq(p, "/") {
		if _, ok := excludedNames[part]; ok {
			return true
		}

		for _, pattern := range excludedPatterns {
			if ok, _ := path.Match(pattern, part); ok {
				return true
			}
		}
	}

	return false
}

func buildTarZst(
	logger *slog.Logger,
	outPath string,
	baseDir string,
	srcPath string,
	arcName string,
	exclude excludeFunc,
) error {
	out, err := os.OpenFile(outPath, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	defer func() { _ = out.Close() }()

	enc, err := zstd.NewWriter(out, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return err
	}

	defer func() { _ = enc.Close() }()

	tw := tar.NewWriter(enc)

	defer func() { _ = tw.Close() }()

	srcAbs := srcPath
	if !filepath.IsAbs(srcAbs) {
		srcAbs = filepath.Join(baseDir, srcAbs)
	}

	srcAbs = filepath.Clean(srcAbs)

	return filepath.WalkDir(srcAbs, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(srcAbs, p)
		if err != nil {
			return err
		}

		name := filepath.ToSlash(filepath.Join(arcName, rel))

		name = strings.TrimSuffix(name, "/.")
		if name == "" || name == "." {
			name = arcName
		}

		if exclude != nil && exclude(name) {
			if d.IsDir() {
				return filepath.SkipDir
			}

			logger.Info("excluding from archive", "path", name)

			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(p)
		if err != nil {
			return err
		}

		defer func() { _ = f.Close() }()

		_, err = io.Copy(tw, f)

		return err
	})
}
