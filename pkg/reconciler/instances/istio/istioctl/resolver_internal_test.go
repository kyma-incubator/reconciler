package istioctl

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Version(t *testing.T) {
	t.Run("should detect it's equal to another instance", func(t *testing.T) {
		given := Version{1, 2, 3}
		another := Version{1, 2, 3}
		require.True(t, given.EqualTo(another))
	})

	t.Run("should detect it's not equal to another instance", func(t *testing.T) {
		given := Version{1, 2, 3}
		another := Version{1, 1, 3}
		require.False(t, given.EqualTo(another))
	})

	t.Run("should detect it is smaller than another instance", func(t *testing.T) {
		given := Version{1, 2, 3}
		another := Version{1, 2, 4}
		require.True(t, given.SmallerThan(another))

		given = Version{1, 2, 3}
		another = Version{1, 3, 3}
		require.True(t, given.SmallerThan(another))

		given = Version{1, 2, 3}
		another = Version{2, 2, 3}
		require.True(t, given.SmallerThan(another))
	})

	t.Run("should detect it is not smaller than another instance", func(t *testing.T) {
		given := Version{1, 2, 4}
		another := Version{1, 2, 3}
		require.False(t, given.SmallerThan(another))

		given = Version{1, 3, 3}
		another = Version{1, 2, 3}
		require.False(t, given.SmallerThan(another))

		given = Version{1, 2, 3}
		equal := Version{1, 2, 3}
		require.False(t, given.SmallerThan(equal))
	})

	t.Run("should detect it is bigger than another instance", func(t *testing.T) {
		given := Version{1, 2, 4}
		another := Version{1, 2, 3}
		require.True(t, given.BiggerThan(another))

		given = Version{1, 3, 3}
		another = Version{1, 2, 3}
		require.True(t, given.BiggerThan(another))

		given = Version{2, 2, 3}
		another = Version{1, 2, 3}
		require.True(t, given.BiggerThan(another))
	})

	t.Run("should detect it is not bigger than another instance", func(t *testing.T) {
		given := Version{1, 2, 3}
		another := Version{1, 2, 4}
		require.False(t, given.BiggerThan(another))

		given = Version{1, 2, 3}
		another = Version{1, 3, 3}
		require.False(t, given.BiggerThan(another))

		given = Version{1, 2, 3}
		equal := Version{1, 2, 3}
		require.False(t, given.BiggerThan(equal))
	})

	t.Run("should serialize to string", func(t *testing.T) {
		given := Version{1, 2, 3}
		asString := fmt.Sprint(given)
		require.Equal(t, "1.2.3", asString)
	})

	t.Run("should deserialize from string", func(t *testing.T) {
		given := Version{1, 11, 4}
		output := fmt.Sprint(given)
		require.Equal(t, "1.11.4", output)
		deserialized, err := VersionFromString(output)
		require.NoError(t, err)
		require.True(t, given.EqualTo(deserialized))
	})

	t.Run("should return error when deserializing from string", func(t *testing.T) {
		_, err := VersionFromString("1.2")
		require.Error(t, err)
		require.Equal(t, "Invalid istioctl version format: \"1.2\"", err.Error())

		_, err = VersionFromString("xyz.2.3")
		require.Error(t, err)
		require.Equal(t, "Invalid istioctl major version: \"xyz\"", err.Error())

		_, err = VersionFromString("1.ijk.2")
		require.Error(t, err)
		require.Equal(t, "Invalid istioctl minor version: \"ijk\"", err.Error())

		_, err = VersionFromString("2.3.abc")
		require.Error(t, err)
		require.Equal(t, "Invalid istioctl patch version: \"abc\"", err.Error())
	})
}

func Test_sortBinaries(t *testing.T) {
	t.Run("should sort binaries in ascending order", func(t *testing.T) {
		given1 := IstioctlBinary{Version{1, 11, 4}, "/biggest"}
		given2 := IstioctlBinary{Version{1, 10, 3}, "/a/b/c"}
		given3 := IstioctlBinary{Version{1, 9, 2}, "/smallest"}
		s := []IstioctlBinary{given1, given2, given3}
		sortBinaries(s)
		require.Equal(t, "/smallest", s[0].path)
		require.Equal(t, "/a/b/c", s[1].path)
		require.Equal(t, "/biggest", s[2].path)
	})
}
