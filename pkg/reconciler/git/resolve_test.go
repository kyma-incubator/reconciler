package git

import (
	"github.com/alcortesm/tgz"
	"github.com/go-git/go-git/v5"
	"os"
	"path"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

type fakeRefLister struct {
	refs []*plumbing.Reference
}

func (fl fakeRefLister) List(repoURL string) ([]*plumbing.Reference, error) {
	return fl.refs, nil
}

var fakeLister refLister = fakeRefLister{[]*plumbing.Reference{plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), plumbing.ZeroHash),
	plumbing.NewHashReference(plumbing.NewBranchReferenceName("testBranch"), plumbing.ZeroHash),
	plumbing.NewHashReference(plumbing.NewTagReferenceName("1.0"), plumbing.ZeroHash),
	plumbing.NewHashReference(plumbing.ReferenceName("refs/pull/9999/head"), plumbing.ZeroHash)}}

func TestResolveRefs(t *testing.T) {
	localRepoRootPath, err := tgz.Extract("testdata/repo.tgz")
	defer func() {
		require.NoError(t, os.RemoveAll(localRepoRootPath))
	}()
	require.NoError(t, err)
	require.NotEmpty(t, localRepoRootPath)

	fakeRepo, err := git.PlainOpen(path.Join(localRepoRootPath, "repo"))
	require.NoError(t, err)

	tests := []struct {
		summary          string
		givenRevision    string
		expectedRevision string
		expectErr        bool
		kind             string
		resolver revisionResolver
	}{
		{
			summary: "pull request uppercase",
			givenRevision:    "PR-9999",
			expectedRevision: plumbing.ZeroHash.String(),
			kind:             "pr",
			resolver: revisionResolver{url: "github.com/fake-repo", repository: fakeRepo, refLister: fakeLister},
		},
		{
			summary: "branch request",
			givenRevision:    "testBranch",
			expectedRevision: plumbing.ZeroHash.String(),
			kind:             "branch",
			resolver: revisionResolver{url: "github.com/fake-repo", repository: fakeRepo, refLister: fakeLister},
		},
		{
			summary: "failing pull request uppercase",
			givenRevision: "PR-1234",
			expectErr:     true,
			kind:          "pr",
			resolver: revisionResolver{url: "github.com/fake-repo", repository: fakeRepo, refLister: fakeLister},
		},
		{
			summary: "failing branch request",
			givenRevision: "nonExistingBranch",
			expectErr:     true,
			kind:          "branch",
			resolver: revisionResolver{url: "github.com/fake-repo", repository: fakeRepo, refLister: fakeLister},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.summary, func(t *testing.T) {
			r, err := tc.resolver.resolveRefs(tc.givenRevision, tc.kind)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedRevision, r)
			}
		})
	}
}
