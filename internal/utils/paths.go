package utils

import (
	"errors"
	"os"
	"path/filepath"
)

type Paths struct {
	BaseDir       string
	ExtensionsDir string
	FlagDir       string
	LogFile       string
	BackupDir     string
}

func ResolveBaseDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	wd, err = filepath.EvalSymlinks(wd)
	if err != nil {
		return "", err
	}

	if wd == "" || wd == string(filepath.Separator) {
		return "", errors.New("invalid working directory")
	}

	return wd, nil
}

func NewPaths(baseDir string) (Paths, error) {
	if baseDir == "" {
		return Paths{}, errors.New("baseDir is empty")
	}

	return Paths{
		BaseDir:       baseDir,
		ExtensionsDir: filepath.Join(baseDir, "extensions"),
		FlagDir:       filepath.Join(baseDir, "flag"),
		LogFile:       filepath.Join(baseDir, "kernel.log"),
		BackupDir:     filepath.Join(baseDir, ".bak"),
	}, nil
}

func ResolveWithinBase(baseDir string, userPath string) (string, error) {
	if userPath == "" {
		return "", errors.New("path is empty")
	}

	candidate := filepath.Clean(userPath)
	if filepath.IsAbs(candidate) {
		return "", errors.New("absolute paths are not allowed")
	}

	full := filepath.Join(baseDir, candidate)

	full, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", err
	}

	baseReal, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(baseReal, full)
	if err != nil {
		return "", err
	}

	if rel == "." {
		return full, nil
	}

	if rel == "" || rel[0] == '.' {
		return "", errors.New("path escapes base directory")
	}

	return full, nil
}
