package progress

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWatchable(t *testing.T) {
	t.Run("Test existing watchables", func(t *testing.T) {
		for _, expected := range []WatchableResource{Deployment, Pod, DaemonSet, StatefulSet, Job} {
			got, err := NewWatchableResource(strings.ToLower(string(expected)))
			require.NoError(t, err)
			require.Equal(t, expected, got)
		}
	})

	t.Run("Test non-existing watchables", func(t *testing.T) {
		_, err := NewWatchableResource("IdontExist")
		require.Error(t, err)
	})
}
