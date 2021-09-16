package git

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

type fakeRefLister struct {
	refs []*plumbing.Reference
}

func (fl *fakeRefLister) List(repoURL string) ([]*plumbing.Reference, error) {
	return fl.refs, nil
}

func TestResolveRefs(t *testing.T) {
	tests := []struct {
		summary          string
		givenRefs        []*plumbing.Reference
		givenRevision    string
		expectedRevision string
		expectErr        bool
		kind             string
	}{
		{
			summary: "pull request uppercase",
			givenRefs: []*plumbing.Reference{
				plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), plumbing.ZeroHash),
				plumbing.NewHashReference(plumbing.NewBranchReferenceName("test-branch"), plumbing.ZeroHash),
				plumbing.NewHashReference(plumbing.NewTagReferenceName("1.0"), plumbing.ZeroHash),
				plumbing.NewHashReference(plumbing.ReferenceName("refs/pull/9999/head"), plumbing.ZeroHash),
			},
			givenRevision:    "PR-9999",
			expectedRevision: plumbing.ZeroHash.String(),
			kind:             "pr",
		},
		{
			summary: "branch request",
			givenRefs: []*plumbing.Reference{
				plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), plumbing.ZeroHash),
				plumbing.NewHashReference(plumbing.NewBranchReferenceName("testBranch"), plumbing.ZeroHash),
				plumbing.NewHashReference(plumbing.NewTagReferenceName("1.0"), plumbing.ZeroHash),
				plumbing.NewHashReference(plumbing.ReferenceName("refs/pull/9999/head"), plumbing.ZeroHash),
			},
			givenRevision:    "testBranch",
			expectedRevision: plumbing.ZeroHash.String(),
			kind:             "branch",
		},
		{
			summary: "failing pull request uppercase",
			givenRefs: []*plumbing.Reference{
				plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), plumbing.ZeroHash),
				plumbing.NewHashReference(plumbing.NewBranchReferenceName("test-branch"), plumbing.ZeroHash),
				plumbing.NewHashReference(plumbing.NewTagReferenceName("1.0"), plumbing.ZeroHash),
				plumbing.NewHashReference(plumbing.ReferenceName("refs/pull/9999/head"), plumbing.ZeroHash),
			},
			givenRevision: "PR-1234",
			expectErr:     true,
			kind:          "pr",
		},
		{
			summary: "failing branch request",
			givenRefs: []*plumbing.Reference{
				plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), plumbing.ZeroHash),
				plumbing.NewHashReference(plumbing.NewBranchReferenceName("test-branch"), plumbing.ZeroHash),
				plumbing.NewHashReference(plumbing.NewTagReferenceName("1.0"), plumbing.ZeroHash),
				plumbing.NewHashReference(plumbing.ReferenceName("refs/pull/9999/head"), plumbing.ZeroHash),
			},
			givenRevision: "nonExistingBranch",
			expectErr:     true,
			kind:          "branch",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.summary, func(t *testing.T) {
			defaultLister = &fakeRefLister{
				refs: tc.givenRefs,
			}
			r, err := resolveRefs("github.com/fake-repo", tc.givenRevision, tc.kind)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedRevision, r)
			}
		})
	}
}
