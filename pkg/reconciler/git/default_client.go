package git

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pkg/errors"
)

type DefaultClient struct {
	repo *git.Repository
}

func (d *DefaultClient) PlainCloneContext(ctx context.Context, path string, isBare bool, o *git.CloneOptions) (*git.Repository, error) {
	var err error
	d.repo, err = git.PlainCloneContext(ctx, path, isBare, o)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("Failed to clone the repository: '%s'", err))
	}
	return d.repo, nil
}

func (d *DefaultClient) Worktree() (*git.Worktree, error) {
	return d.repo.Worktree()
}

func (d *DefaultClient) ResolveRevision(rev plumbing.Revision) (*plumbing.Hash, error) {
	return d.repo.ResolveRevision(rev)
}
