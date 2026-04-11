package utils

import (
	"errors"
	"log/slog"
	"path/filepath"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type RepoInfo struct {
	URL                 string
	LocalCommitID       string
	RemoteCommitID      string
	LocalCommitTimeUTC  string
	RemoteCommitTimeUTC string
	UncommittedChanges  int
	UpToDate            bool
}

func GetKernelInfo(baseDir string) (RepoInfo, error) {
	repo, err := git.PlainOpenWithOptions(baseDir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return RepoInfo{}, err
	}

	remoteURL, _ := originURL(repo)

	wt, err := repo.Worktree()
	if err != nil {
		return RepoInfo{}, err
	}

	status, err := wt.Status()
	if err != nil {
		return RepoInfo{}, err
	}

	uncommitted := 0

	for _, st := range status {
		if st.Worktree != git.Unmodified || st.Staging != git.Unmodified {
			uncommitted++
		}
	}

	headRef, err := repo.Head()
	if err != nil {
		return RepoInfo{}, err
	}

	localCommit, _ := repo.CommitObject(headRef.Hash())

	remoteCommit, err := fetchOriginAndResolve(repo)
	if err != nil {
		return RepoInfo{}, err
	}

	info := RepoInfo{
		URL:                remoteURL,
		LocalCommitID:      headRef.Hash().String(),
		RemoteCommitID:     remoteCommit.Hash.String(),
		UncommittedChanges: uncommitted,
		UpToDate:           headRef.Hash() == remoteCommit.Hash,
	}
	if localCommit != nil {
		info.LocalCommitTimeUTC = localCommit.Committer.When.UTC().Format(time.RFC3339Nano)
	}

	if remoteCommit.Commit != nil {
		info.RemoteCommitTimeUTC = remoteCommit.Commit.Committer.When.UTC().Format(time.RFC3339Nano)
	}

	return info, nil
}

type resolvedCommit struct {
	Commit *object.Commit
	Hash   plumbing.Hash
}

func fetchOriginAndResolve(repo *git.Repository) (resolvedCommit, error) {
	_ = repo.Fetch(&git.FetchOptions{RemoteName: "origin", Force: true, Tags: git.NoTags})

	refNames := []plumbing.ReferenceName{
		"refs/remotes/origin/HEAD",
		"refs/remotes/origin/main",
		"refs/remotes/origin/master",
	}
	for _, name := range refNames {
		ref, err := repo.Reference(name, true)
		if err != nil {
			continue
		}

		hash := ref.Hash()
		if name == "refs/remotes/origin/HEAD" {
			if ref.Type() == plumbing.SymbolicReference {
				target := ref.Target()
				if target != "" {
					if targetRef, err := repo.Reference(target, true); err == nil {
						hash = targetRef.Hash()
					}
				}
			}
		}

		commit, _ := repo.CommitObject(hash)

		return resolvedCommit{Hash: hash, Commit: commit}, nil
	}

	return resolvedCommit{}, errors.New("failed to resolve origin ref")
}

func originURL(repo *git.Repository) (string, bool) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", false
	}

	cfg := remote.Config()
	if cfg == nil || len(cfg.URLs) == 0 {
		return "", false
	}

	return cfg.URLs[0], true
}

func PullToRemote(logger *slog.Logger, baseDir string) error {
	repo, err := git.PlainOpenWithOptions(baseDir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return err
	}

	remoteCommit, err := fetchOriginAndResolve(repo)
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	if err := wt.Reset(
		&git.ResetOptions{Mode: git.HardReset, Commit: remoteCommit.Hash},
	); err != nil {
		return err
	}

	head, err := repo.Head()
	if err != nil {
		return err
	}

	logger.Info(
		"kernel pulled",
		"from",
		head.Hash().String(),
		"to",
		remoteCommit.Hash.String(),
		"dir",
		filepath.Clean(baseDir),
	)

	return nil
}
