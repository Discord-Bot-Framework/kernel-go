package modules

import (
	"context"
	"errors"
	"fmt"

	git "github.com/go-git/go-git/v5"
)

func Clone(ctx context.Context, url string, dir string) error {
	_, err := git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{
		URL:               url,
		RecurseSubmodules: git.NoRecurseSubmodules,
		Depth:             1,
		SingleBranch:      true,
		NoCheckout:        false,
	})
	if err != nil {
		return fmt.Errorf("clone repo: %w", err)
	}

	return nil
}

func Pull(ctx context.Context, dir string) error {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}

	err = wt.PullContext(ctx, &git.PullOptions{
		RemoteName: "origin",
		Force:      true,
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("pull repo: %w", err)
	}

	return nil
}
