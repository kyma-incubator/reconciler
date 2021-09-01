package git

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/pkg/errors"
)

type RemoteRepoCloner struct {
	repo         *reconciler.Repository
	auth         transport.AuthMethod
	autoCheckout bool

	repoClient RepoClient
}

//go:generate mockery --name RepoClient --case=underscore
type RepoClient interface {
	PlainCloneContext(ctx context.Context, path string, isBare bool, o *git.CloneOptions) (*git.Repository, error)
	Worktree() (*git.Worktree, error)
	ResolveRevision(rev plumbing.Revision) (*plumbing.Hash, error)
}

func NewCloner(repoClient RepoClient, repo *reconciler.Repository, autoCheckout bool) *RemoteRepoCloner {
	var auth transport.AuthMethod
	if repo != nil && repo.Token != "" {
		auth = &http.BasicAuth{
			Username: "xxx", // anything but an empty string
			Password: repo.Token,
		}
	}

	return &RemoteRepoCloner{
		repo:         repo,
		auth:         auth,
		autoCheckout: autoCheckout,

		repoClient: repoClient,
	}
}

// Clone clones the repository from the given remote URL to the given `path` in the local filesystem.
func (r *RemoteRepoCloner) Clone(path string) error {
	var err error
	_, err = r.repoClient.PlainCloneContext(context.Background(), path, false, &git.CloneOptions{
		Depth:             0,
		URL:               r.repo.URL,
		NoCheckout:        !r.autoCheckout,
		Auth:              r.auth,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})

	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to clone the repository '%s'", err))
	}

	return nil
}

// Checkout checks out the given revision.
// revision can be 'main', a release version (e.g. 1.4.1), a commit hash (e.g. 34edf09a).
func (r *RemoteRepoCloner) Checkout(rev string) error {
	w, err := r.repoClient.Worktree()
	if err != nil {
		return errors.Wrap(err, "error getting the GIT worktree")
	}

	hash, err := r.repoClient.ResolveRevision(plumbing.Revision(rev))
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

func (r *RemoteRepoCloner) CloneAndCheckout(dstPath, rev string) error {
	if rev == "" {
		return fmt.Errorf("GIT revision cannot be empty")
	}
	err := r.Clone(dstPath)
	if err != nil {
		return errors.Wrapf(err, "Error downloading Git repository (%s)", r.repo)
	}

	return r.Checkout(rev)
}
