package git

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/alcortesm/tgz"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/stretchr/testify/require"
)

// TestClient tests CloneAndCheckout function that is provided with a dummy git repository (no actual cloning is performed)
// The repo has following commits
// 1. Add README (tagged with 1.0.0)
// 2. Update README (tagged with 2.0.0 - HEAD)
func TestClient(t *testing.T) {

	t.Run("Should clone data", func(t *testing.T) {
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

		headRef, err := repo.Head()
		require.NoError(t, err)

		commit, err := repo.CommitObject(headRef.Hash())
		require.NoError(t, err)
		require.Equal(t, "Update README\n", commit.Message)

		client := &Client{}

		repo, err = client.Clone(context.Background(),
			localRepoRootPath+"../", false, &git.CloneOptions{
				Depth:             0,
				URL:               "file:///" + localRepoRootPath + "/repo/.git",
				NoCheckout:        false,
				Auth:              transport.AuthMethod(nil),
				RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
			})

		require.NoError(t, err)

		_, err = repo.Head()
		require.NoError(t, err)
	})
}
