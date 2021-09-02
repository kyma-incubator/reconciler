package git

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/git/mocks"
	"github.com/stretchr/testify/assert"

	"github.com/alcortesm/tgz"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

// TestCloneRepo tests CloneAndCheckout function that is provided with a dummy git repository (no actual cloning is performed)
// The repo has following commits
// 1. Add README (tagged with 1.0.0)
// 2. Update README (tagged with 2.0.0 - HEAD)
func TestCloneRepo(t *testing.T) {
	localRepoRootPath, err := tgz.Extract("testdata/repo.tgz")
	defer func() {
		require.NoError(t, os.RemoveAll(localRepoRootPath))
	}()
	require.NoError(t, err)
	require.NotEmpty(t, localRepoRootPath)

	repo, err := git.PlainOpen(path.Join(localRepoRootPath, "repo"))
	require.NoError(t, err)

	var refs []*plumbing.Reference
	iter, err := repo.References()
	require.NoError(t, err)

	err = iter.ForEach(func(r *plumbing.Reference) error {
		refs = append(refs, r)
		return nil
	})
	require.NoError(t, err)

	clonerMock := &mocks.RepoClient{}

	repoUrl := "github.com/foo"
	r := reconciler.Repository{URL: repoUrl}
	options := git.CloneOptions{
		Depth:             0,
		URL:               r.URL,
		NoCheckout:        false,
		Auth:              transport.AuthMethod(nil),
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	}
	clonerMock.On("Clone",
		context.Background(), "bar/baz", false, &options).
		Return(repo, nil)
	clonerMock.On("Worktree").
		Return(repo.Worktree())
	clonerMock.On("ResolveRevision",
		plumbing.Revision("1.0.0")).
		Return(repo.ResolveRevision("1.0.0"))
	cloner := NewCloner(clonerMock, &r, true)

	headRef, err := repo.Head()
	require.NoError(t, err)

	commit, err := repo.CommitObject(headRef.Hash())
	require.NoError(t, err)
	require.Equal(t, "Update README\n", commit.Message)

	err = cloner.CloneAndCheckout("bar/baz", "1.0.0")
	require.NoError(t, err)

	headRef, err = repo.Head()
	require.NoError(t, err)

	commit, err = repo.CommitObject(headRef.Hash())
	require.NoError(t, err)
	require.Equal(t, "Add README\n", commit.Message)

	t.Run("Should add auth data if token set", func(t *testing.T) {
		token := "tokeValue"
		//client := mocks.RepoClient{}
		autoCheckout := false
		clonerMock.On("Clone", context.Background(), "/test", autoCheckout, &git.CloneOptions{
			Depth:      0,
			URL:        repoUrl,
			NoCheckout: !autoCheckout,
			Auth: &http.BasicAuth{
				Username: "xxx", // anything but an empty string
				Password: token,
			},
			RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		}).Return(repo, nil)
		cloner := NewCloner(clonerMock, &reconciler.Repository{
			URL:   repoUrl,
			Token: token,
		}, autoCheckout)

		err := cloner.Clone("/test")
		assert.NoError(t, err)
	})
}
