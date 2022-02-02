package db

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTransaction(t *testing.T) {

	t.Run("Test random jitter", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			jitter := randomJitter().Milliseconds()
			require.True(t, jitter >= txMinJitter && jitter <= txMaxJitter)
		}
	})
}
