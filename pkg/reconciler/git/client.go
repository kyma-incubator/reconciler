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
	if _, err := d.repo.Branch(rev.String()); err != git.ErrBranchNotFound {
		return d.repo.ResolveRevision(gitp.Revision(fmt.Sprintf("refs/remotes/origin/%s", rev.String())))
	}
	return d.repo.ResolveRevision(rev)
}

func (d *Client) Fetch(path string, o *git.FetchOptions) error {
	var err error
	if d.repo == nil {
		d.repo, err = git.PlainOpen(path)
		if err != nil {
			return err
		}
	}
	err = d.repo.Fetch(o)
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

func NewClientWithPath(path string) (*Client, error) {
	var err error
	d := &Client{}
	d.repo, err = git.PlainOpen(path)
	return d, err
}

// func (d *Client) ResolveRevisionOrHead(rev gitp.Revision) (*gitp.Hash, error) {
// 	if _, err := d.repo.Branch(rev.String()); err == git.ErrBranchNotFound {
// 		return d.repo.ResolveRevision(rev)// not a branch
// 	}
// 	return d.repo.ResolveRevision(rev)
// }

func (d *Client) Repo() *git.Repository {
	return d.repo
}
