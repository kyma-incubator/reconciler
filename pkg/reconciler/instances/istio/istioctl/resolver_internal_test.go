package istioctl

import (
	"fmt"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/helpers"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	t.Run("should parse release version", func(t *testing.T) {
		given, err := helpers.NewHelperVersionFrom("istio/proxyv2:1.11.3")
		require.NoError(t, err)
		output := fmt.Sprint(given.Tag)
		require.Equal(t, "1.11.3", output)
	})

	t.Run("should detect it's equal to another instance", func(t *testing.T) {
		given, _ := helpers.NewHelperVersionFrom("1.2.3")
		another, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		require.True(t, given.EqualTo(*another))
	})

	t.Run("should detect it's not equal to another instance", func(t *testing.T) {
		given, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		another, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.1.3")
		require.False(t, given.EqualTo(*another))
	})

	t.Run("should detect it is smaller than another instance", func(t *testing.T) {

		given, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		another, _ := helpers.NewHelperVersionFrom("2.2.3")
		require.True(t, given.SmallerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.3.3")
		require.True(t, given.SmallerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.4")
		require.True(t, given.SmallerThan(*another))

	})

	t.Run("should detect it is not smaller than another instance", func(t *testing.T) {
		given, _ := helpers.NewHelperVersionFrom("istio/proxyv2:2.2.3")
		another, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		require.False(t, given.SmallerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.3.3")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		require.False(t, given.SmallerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.4")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		require.False(t, given.SmallerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		equal, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		require.False(t, given.SmallerThan(*equal))

	})

	t.Run("should detect it is bigger than another instance", func(t *testing.T) {

		given, _ := helpers.NewHelperVersionFrom("istio/proxyv2:2.2.3")
		another, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		require.True(t, given.BiggerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.3.3")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		require.True(t, given.BiggerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.4")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		require.True(t, given.BiggerThan(*another))
	})

	t.Run("should detect it is not bigger than another instance", func(t *testing.T) {
		given, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		another, _ := helpers.NewHelperVersionFrom("istio/proxyv2:2.2.3")
		require.False(t, given.BiggerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.3.3")
		require.False(t, given.BiggerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.4")
		require.False(t, given.BiggerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		equal, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3")
		require.False(t, given.BiggerThan(*equal))
	})

	t.Run("should return error when deserializing from invalid string", func(t *testing.T) {
		_, err := helpers.NewHelperVersionFrom("istio/proxyv2:1.2")
		require.Error(t, err)

		_, err = helpers.NewHelperVersionFrom("istio/proxyv2:xyz.2.3")
		require.Error(t, err)

		_, err = helpers.NewHelperVersionFrom("istio/proxyv2:1.ijk.2")
		require.Error(t, err)

		_, err = helpers.NewHelperVersionFrom("istio/proxyv2:2.3.abc")
		require.Error(t, err)
	})

}

func TestBetaVersion(t *testing.T) {

	t.Run("should parse a beta version", func(t *testing.T) {
		given, err := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		require.NoError(t, err)

		output := fmt.Sprint(given.Tag)
		require.Equal(t, "1.2.3-beta.1", output)

		stillValid, err := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-")
		require.NoError(t, err)
		require.Equal(t, "1.2.3", fmt.Sprint(stillValid.Tag))
	})

	t.Run("should detect it's equal to another instance", func(t *testing.T) {
		given, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		another, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		require.True(t, given.EqualTo(*another))
	})

	t.Run("should detect it's not equal to another instance", func(t *testing.T) {
		given, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		another, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.2")
		require.False(t, given.EqualTo(*another))
	})

	t.Run("should detect it is smaller than another instance", func(t *testing.T) {
		given, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-alpha.1")
		another, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		require.True(t, given.SmallerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.0")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		require.True(t, given.SmallerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.2")
		require.True(t, given.SmallerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.9")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.10")
		require.True(t, given.SmallerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-rc.1")
		require.True(t, given.SmallerThan(*another))
	})

	t.Run("should detect it is not smaller than another instance", func(t *testing.T) {
		given, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		another, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-alpha.1")
		require.False(t, given.SmallerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.0")
		require.False(t, given.SmallerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.2")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		require.False(t, given.SmallerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.10")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.9")
		require.False(t, given.SmallerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-rc.1")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		require.False(t, given.SmallerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-rc.1")
		equal, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-rc.1")
		require.False(t, given.SmallerThan(*equal))
	})

	t.Run("should detect it is bigger than another instance", func(t *testing.T) {
		given, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		another, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-alpha.1")
		require.True(t, given.BiggerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.0")
		require.True(t, given.BiggerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.2")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		require.True(t, given.BiggerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.10")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.9")
		require.True(t, given.BiggerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-rc.1")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		require.True(t, given.BiggerThan(*another))
	})

	t.Run("should detect it is not bigger than another instance", func(t *testing.T) {

		given, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-alpha.1")
		another, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		require.False(t, given.BiggerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.0")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		require.False(t, given.BiggerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.2")
		require.False(t, given.BiggerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.9")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.10")
		require.False(t, given.BiggerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-beta.1")
		another, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-rc.1")
		require.False(t, given.BiggerThan(*another))

		given, _ = helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-rc.1")
		equal, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3-rc.1")
		require.False(t, given.BiggerThan(*equal))
	})

	t.Run("should return error when deserializing from invalid string", func(t *testing.T) {
		_, err := helpers.NewHelperVersionFrom("istio/proxyv2:1.2.3.-abc.")
		require.Error(t, err)
	})
}

func TestSortBinaries(t *testing.T) {
	t.Run("should sort binaries in ascending order", func(t *testing.T) {
		v1, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.11.4")
		v2, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.10.3")
		v3, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.9.2")
		v4, _ := helpers.NewHelperVersionFrom("istio/proxyv2:1.9.2-beta.1")
		given1 := Executable{*v1, "/biggest"}
		given2 := Executable{*v2, "/a/b/c"}
		given3 := Executable{*v3, "/smallest"}
		given4 := Executable{*v4, "/smallest-beta"}
		s := []Executable{given1, given2, given4, given3}
		sortBinaries(s)
		require.Equal(t, "/smallest-beta", s[0].path)
		require.Equal(t, "/smallest", s[1].path)
		require.Equal(t, "/a/b/c", s[2].path)
		require.Equal(t, "/biggest", s[3].path)
	})
}
