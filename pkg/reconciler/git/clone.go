package git

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
)

var defaultCloner repoCloner = &remoteRepoCloner{}

// CloneRepo clones the repository in the given URL to the given dstPath and checks out the given revision.
// revision can be 'main', a release version (e.g. 1.4.1), a commit hash (e.g. 34edf09a).
func CloneRepo(url, dstPath, rev string) error {
	if rev == "" {
		return fmt.Errorf("GIT revision cannot be empty")
	}
	repo, err := defaultCloner.Clone(url, dstPath, true)
	if err != nil {
		return errors.Wrapf(err, "error downloading Git repository (%s)", url)
	}
	return checkout(repo, rev, url)
}

type repoCloner interface {
	Clone(url, path string, noCheckout bool) (*git.Repository, error)
}

type remoteRepoCloner struct {
}

func (rc *remoteRepoCloner) Clone(url, path string, autoCheckout bool) (*git.Repository, error) {
	return git.PlainCloneContext(context.Background(), path, false, &git.CloneOptions{
		Depth:      0,
		URL:        url,
		NoCheckout: !autoCheckout,
	})
}

func checkout(repo *git.Repository, rev string, url string) error {
	w, err := repo.Worktree()
	if err != nil {
		return errors.Wrap(err, "error getting the GIT worktree")
	}

	hash, err := resolveRevision(repo, url, rev)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to resolve GIT revision '%s'", rev))
	}
	err = w.Checkout(&git.CheckoutOptions{
		Hash: *hash,
	})
	if err != nil {
		return errors.Wrap(err, "Error checking out GIT revision")
	}
	return nil
}
