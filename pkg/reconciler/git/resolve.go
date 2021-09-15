package git

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pkg/errors"
)

type refLister interface {
	List(repoURL string) ([]*plumbing.Reference, error)
}

type remoteRefLister struct {
}

var defaultLister refLister = &remoteRefLister{}

const prPrefix string = "PR-"

func (rl *remoteRefLister) List(repoURL string) ([]*plumbing.Reference, error) {
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	})

	return remote.List(&git.ListOptions{})
}

// revision can be 'main', a release version (e.g. 1.4.1), a commit hash (e.g. 34edf09a) or a PR (e.g. PR-9486).
func resolveRevision(repo *git.Repository, url, rev string) (*plumbing.Hash, error) {
	if strings.HasPrefix(rev, prPrefix) {
		err := fetchPR(repo, strings.TrimPrefix(rev, prPrefix)) // to ensure that the rev hash can be checked out
		if err != nil {
			return nil, err
		}
		rev, err = resolvePRrevision(url, rev)
		if err != nil {
			return nil, err
		}
	}
	return repo.ResolveRevision(plumbing.Revision(rev))
}

func fetchPR(repo *git.Repository, prNmbr string) error {
	refs := []config.RefSpec{config.RefSpec(fmt.Sprintf("+refs/pull/%s/head:refs/remotes/origin/pr/%s", prNmbr, prNmbr))}
	return repo.Fetch(&git.FetchOptions{RefSpecs: refs})
}

// resolvePRrevision tries to convert a PR into a revision that can be checked out.
func resolvePRrevision(repoURL, pr string) (string, error) {
	refs, err := defaultLister.List(repoURL)
	if err != nil {
		return "", errors.Wrap(err, "could not list commits")
	}

	if strings.HasPrefix(pr, prPrefix) {
		pr = strings.TrimLeft(pr, prPrefix)
	}

	for _, ref := range refs {
		if strings.HasPrefix(ref.Name().String(), "refs/pull") && strings.HasSuffix(ref.Name().String(), "head") && strings.Contains(ref.Name().String(), pr) {
			return ref.Hash().String(), nil
		}
	}
	return "", errors.Errorf("could not find HEAD of pull request %s in %s", pr, repoURL)
}
