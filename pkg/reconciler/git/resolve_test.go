package git

import (
	"encoding/hex"
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

func TestResolvePRrevision(t *testing.T) {
	tests := []struct {
		summary       string
		givenRefs     []*plumbing.Reference
		givenRevision string
		expectErr     bool
	}{
		{
			summary: "pull request uppercase",
			givenRefs: []*plumbing.Reference{
				plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), plumbing.ZeroHash),
				plumbing.NewHashReference(plumbing.NewTagReferenceName("1.0"), plumbing.ZeroHash),
				plumbing.NewHashReference(plumbing.ReferenceName("refs/pull/9999/head"), plumbing.ZeroHash),
			},
			givenRevision: "PR-9999",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.summary, func(t *testing.T) {
			defaultLister = &fakeRefLister{
				refs: tc.givenRefs,
			}
			r, err := resolveRefs("github.com/fake-repo", tc.givenRevision, "pr")
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.True(t, isHex(r))
			}
		})
	}
}

func isHex(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil && len(s) > 7
}
