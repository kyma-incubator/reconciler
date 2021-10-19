package istioctl

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_IstioVersion(t *testing.T) {
	t.Run("should detect it's equal to another instance", func(t *testing.T) {
		given := IstioVersion{1, 2, 3}
		another := IstioVersion{1, 2, 3}
		require.True(t, given.EqualTo(another))
	})
	t.Run("should detect it's not equal to another instance", func(t *testing.T) {
		given := IstioVersion{1, 2, 3}
		another := IstioVersion{1, 1, 3}
		require.False(t, given.EqualTo(another))
	})
	t.Run("should detect it is smaller than another instance", func(t *testing.T) {})
	t.Run("should detect it is not smaller than another instance", func(t *testing.T) {})
	t.Run("should detect it is bigger than another instance", func(t *testing.T) {})
	t.Run("should detect it is not bigger than another instance", func(t *testing.T) {})

	t.Run("should serialize to string", func(t *testing.T) {
		given := IstioVersion{1, 2, 3}
		asString := fmt.Sprint(given)
		require.Equal(t, "1.2.3", asString)
	})

	t.Run("should deserialize from string", func(t *testing.T) {
		given := IstioVersion{1, 2, 0}
		output := fmt.Sprint(given)
		require.Equal(t, "1.2.0", output)
		deserialized, err := istioVersionFromString(output)
		require.NoError(t, err)
		require.True(t, given.EqualTo(deserialized))
	})
}
