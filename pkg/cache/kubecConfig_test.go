package cache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetJitterTTL(t *testing.T) {
	ttl := getTTL()
	minMinutes := int64(ttl.Minutes())
	maxMinutes := int64(ttl.Minutes()) + int64(ttl.Minutes()/3)
	for i := 0; i < 100; i++ {
		jitter := int64(getJitterTTL().Minutes())
		require.True(t, jitter >= minMinutes && jitter <= maxMinutes,
			"Value of %dmin is out of min-max range: %d - %d", jitter, minMinutes, maxMinutes)
	}
}
