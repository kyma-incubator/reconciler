package cache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetJitterTTL(t *testing.T) {
	maxTTL := getTTL()
	minMinutes := int64(maxTTL.Minutes() / 4)
	maxMinutes := int64(maxTTL.Minutes())
	for i := 0; i < 100; i++ {
		randDurationMin := int64(getJitterTTL().Minutes())
		require.True(t, randDurationMin <= maxMinutes && randDurationMin >= minMinutes,
			"Value of %dmin is out of min-max range: %d - %d", randDurationMin, minMinutes, maxMinutes)
		t.Log(randDurationMin)
	}
}
