package git

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5"
	gitp "github.com/go-git/go-git/v5/plumbing"
	"github.com/pkg/errors"
)

type Client struct {
	repo *git.Repository
}

func (d *Client) Clone(ctx context.Context, path string, isBare bool, o *git.CloneOptions) (*git.Repository, error) {
	var err error
	d.repo, err = git.PlainCloneContext(ctx, path, isBare, o)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("Failed to clone the repository: '%s'", err))
	}
	return d.repo, nil
}

func (d *Client) Worktree() (*git.Worktree, error) {
	return d.repo.Worktree()
}

func (d *Client) ResolveRevisionOrBranchHead(rev gitp.Revision) (*gitp.Hash, error) {
	// no version provided. We will set rev to the default branch name.
	if rev.String() == "" {
		branch, err := d.DefaultBranch()
		if err != nil {
			return nil, err
		}
		rev = gitp.Revision(branch.Name().Short())
	}
	// is rev a branch name? We can't use repo.Branch() because that checks local branches only :/
	// So, we try to resolve rev as a remote branch reference. If that works, we
	// can just use it 's head!
	branchRev, err := d.repo.ResolveRevision(
		gitp.Revision(
			fmt.Sprintf("refs/remotes/origin/%s", rev.String()),
		),
	)
	if err != nil && err != gitp.ErrReferenceNotFound {
		return nil, err
	}
	if !branchRev.IsZero() { // this is a branch
		return d.repo.ResolveRevision(gitp.Revision(branchRev.String()))
	}
	return d.repo.ResolveRevision(rev)
}

func (d *Client) Fetch(o *git.FetchOptions) error {
	if d.repo == nil {
		return errors.New("repo is not initialized")
	}
	err := d.repo.Fetch(o)
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

func (d *Client) PlainCheckout(o *git.CheckoutOptions) error {
	var err error
	if d.repo == nil {
		return errors.New("repo is not initialized")
	}
	w, err := d.repo.Worktree()
	if err != nil {
		return err
	}
	return w.Checkout(o)
}

func NewClientWithPath(path string) (RepoClient, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	return &Client{repo: repo}, nil
}

func (d *Client) DefaultBranch() (*gitp.Reference, error) {
	if d.repo == nil {
		return nil, errors.New("repo is not initialized")
	}
	branches, err := d.repo.Branches()
	if err != nil {
		return nil, err
	}
	// the repository default branch is the one we get when it's cloned.
	return branches.Next()
}
