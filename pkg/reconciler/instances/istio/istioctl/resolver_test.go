package istioctl_test

import (
	"os"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl/mocks"
	"github.com/stretchr/testify/require"
)

func Test_DefaultIstioctlResolver(t *testing.T) {
	t.Run("should match exact version if it exists", func(t *testing.T) {
		vc := mocks.VersionChecker{}

		vc.On("GetIstioVersion", "/d").Return(istioctl.VersionFromString("1.2.1"))
		vc.On("GetIstioVersion", "/c").Return(istioctl.VersionFromString("1.2.4"))
		vc.On("GetIstioVersion", "/b").Return(istioctl.VersionFromString("1.2.7"))
		vc.On("GetIstioVersion", "/a").Return(istioctl.VersionFromString("1.11.2"))

		paths := []string{"/a", "/b", "/c", "/d"}
		resolver, err := istioctl.NewDefaultIstioctlResolver(paths, &vc)
		require.NoError(t, err)

		actualVersion, err := istioctl.VersionFromString("1.2.4")
		require.NoError(t, err)

		binary, err := resolver.FindIstioctl(actualVersion)
		require.NoError(t, err)
		require.Equal(t, "/c", binary.Path())
		require.Equal(t, "1.2.4", binary.Version().String())

		actualVersion, err = istioctl.VersionFromString("1.11.2")
		require.NoError(t, err)

		binary, err = resolver.FindIstioctl(actualVersion)
		require.NoError(t, err)
		require.Equal(t, "/a", binary.Path())
		require.Equal(t, "1.11.2", binary.Version().String())
	})

	t.Run("should match a biggest patch version if no exact match exists", func(t *testing.T) {
		vc := mocks.VersionChecker{}

		vc.On("GetIstioVersion", "/d").Return(istioctl.VersionFromString("1.2.1"))
		vc.On("GetIstioVersion", "/c").Return(istioctl.VersionFromString("1.2.4"))
		vc.On("GetIstioVersion", "/b").Return(istioctl.VersionFromString("1.2.7"))
		vc.On("GetIstioVersion", "/a").Return(istioctl.VersionFromString("1.11.2"))

		paths := []string{"/a", "/b", "/c", "/d"}
		resolver, err := istioctl.NewDefaultIstioctlResolver(paths, &vc)
		require.NoError(t, err)

		actualVersion, err := istioctl.VersionFromString("1.2.3")
		require.NoError(t, err)

		binary, err := resolver.FindIstioctl(actualVersion)
		require.NoError(t, err)
		require.Equal(t, "/b", binary.Path())
	})

	t.Run("should return an error when no match exists", func(t *testing.T) {
		vc := mocks.VersionChecker{}

		vc.On("GetIstioVersion", "/d").Return(istioctl.VersionFromString("1.2.1"))
		vc.On("GetIstioVersion", "/c").Return(istioctl.VersionFromString("1.2.4"))
		vc.On("GetIstioVersion", "/b").Return(istioctl.VersionFromString("1.2.7"))
		vc.On("GetIstioVersion", "/a").Return(istioctl.VersionFromString("1.11.2"))

		paths := []string{"/a", "/b", "/c", "/d"}
		resolver, err := istioctl.NewDefaultIstioctlResolver(paths, &vc)
		require.NoError(t, err)

		actualVersion, err := istioctl.VersionFromString("1.3.0")
		require.NoError(t, err)

		_, err = resolver.FindIstioctl(actualVersion)
		require.Error(t, err)
		require.Equal(t, "No matching Istioctl binary found for version: 1.3.0", err.Error())
	})
}

func Test_DefaultVersionChecker(t *testing.T) {
	t.Run("should return istioctl version from actual invocation", func(t *testing.T) {
		t.Skip("MANUAL TEST!")

		//given
		vc := istioctl.DefaultVersionChecker{}
		path := os.Getenv("ISTIOCTL_BINARY_PATH_TEST")
		require.NotEmpty(t, path)

		expectedVersion := os.Getenv("ISTIOCTL_BINARY_VERSION_TEST")
		require.NotEmpty(t, expectedVersion)

		//when
		version, err := vc.GetIstioVersion(path)

		//then
		require.NoError(t, err)
		require.Equal(t, expectedVersion, version.String())
	})

}
