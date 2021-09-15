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

// revision can be 'main', a branch name, a release version (e.g. 1.4.1), a commit hash (e.g. 34edf09a) or a PR (e.g. PR-9486).
func resolveRevision(repo *git.Repository, url, rev string) (*plumbing.Hash, error) {
	// Check if revision is PR
	if strings.HasPrefix(rev, prPrefix) {
		if err := fetch(repo, strings.TrimPrefix(rev, prPrefix), "pr"); err != nil { // to ensure that the rev hash can be checked out
			return nil, err
		}
		rev, err := resolveRefs(url, strings.TrimLeft(rev, prPrefix), "pr")
		if err != nil {
			return nil, err
		}
		return repo.ResolveRevision(plumbing.Revision(rev))
	}
	// Check if revision is branch
	if err := fetch(repo, rev, "branch"); err == nil {
		rev, err := resolveRefs(url, rev, "branch")
		if err != nil {
			return nil, err
		}
		return repo.ResolveRevision(plumbing.Revision(rev))
	}
	// Revision is main, release version or commit hash
	return repo.ResolveRevision(plumbing.Revision(rev))
}

func fetch(repo *git.Repository, name string, kind string) error {
	switch kind {
	case "pr":
		refs := []config.RefSpec{config.RefSpec(fmt.Sprintf("+refs/pull/%s/head:refs/remotes/origin/pr/%s", name, name))}
		return repo.Fetch(&git.FetchOptions{RefSpecs: refs})

	case "branch":
		refs := []config.RefSpec{config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", name, name))}
		return repo.Fetch(&git.FetchOptions{RefSpecs: refs})

	default:
		return errors.Errorf("Unknown Type: %s", kind)
	}
}

// resolvePRrevision tries to convert a PR into a revision that can be checked out.
func resolveRefs(repoURL, name string, kind string) (string, error) {
	refs, err := defaultLister.List(repoURL)
	if err != nil {
		return "", errors.Wrap(err, "could not list commits")
	}

	switch kind {
	case "pr":
		for _, ref := range refs {
			if strings.HasPrefix(ref.Name().String(), "refs/pull") && strings.HasSuffix(ref.Name().String(), "head") && strings.Contains(ref.Name().String(), name) {
				return ref.Hash().String(), nil
			}
		}
		return "", errors.Wrapf(err, "could not find HEAD of pull request %s in %s", name, repoURL)

	case "branch":
		for _, ref := range refs {
			if strings.HasPrefix(ref.Name().String(), "refs/heads") && strings.Contains(ref.Name().String(), name) {
				return ref.Hash().String(), nil
			}
		}
		return "", errors.Wrapf(err, "could not find HEAD of branch %s in %s", name, repoURL)

	default:
		return "", errors.Errorf("Unknown type: %s", kind)
	}

}
