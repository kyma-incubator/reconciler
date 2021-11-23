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

func (d *Client) ResolveRevision(rev gitp.Revision) (*gitp.Hash, error) {
	return d.repo.ResolveRevision(rev)
}

func (d *Client) PlainOpen(path string) error {
	var err error
	d.repo, err = git.PlainOpen(path)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to open the repository at [%s]: '%s'", path, err))
	}
	return nil
}
