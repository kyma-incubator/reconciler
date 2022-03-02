package db

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPostgresContainer(t *testing.T) {
	ctx := context.Background()
	c, _ := NewTestPostgresContainer(t, true, true)
	t.Cleanup(func() {
		require.NoError(t, c.Terminate(ctx))
	})

	stateAfterStart, startStateFetchError := c.State(ctx)
	require.NoError(t, startStateFetchError, "r should have stateAfterStart")

	require.True(t, stateAfterStart.Running, "r should be running")
}
