package workspace

import (
	"os"
	"path/filepath"
	"testing"

	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

const version = "1.20.0"

func TestWorkspaceFactory(t *testing.T) {

	t.Run("Test getter", func(t *testing.T) {
		wsf1 := Factory{}
		defaultStorageDir := wsf1.defaultStorageDir()
		expVersionDir1 := filepath.Join(defaultStorageDir, version)
		require.Equal(t, expVersionDir1, wsf1.versionDir(version))

		wsf2 := Factory{
			StorageDir: "/tmp",
		}
		expVersionDir2 := filepath.Join("/tmp", version)
		require.Equal(t, expVersionDir2, wsf2.versionDir(version))
	})

	t.Run("Clone workspace", func(t *testing.T) {
		if !test.RunExpensiveTests() {
			//this test case clones the Kyma repo can take up to 60 sec (depending on the bandwidth) and generates bigger amount of traffic
			return
		}

		versionDir := filepath.Join(".", "test", version)

		//cleanup at the beginning (if test was interrupted before)
		cleanup(t, versionDir)
		//cleanup at the end (if test finishes regularly)
		defer cleanup(t, versionDir)

		wsf := Factory{
			StorageDir: "./test",
		}

		ws, err := wsf.Get(version)
		require.NoError(t, err)

		require.Equal(t, filepath.Join(versionDir, componentFile), ws.ComponentFile)
		require.True(t, file.Exists(ws.ComponentFile))
		require.Equal(t, filepath.Join(versionDir, resDir), ws.ResourceDir)
		require.True(t, file.DirExists(ws.ResourceDir))
		require.Equal(t, filepath.Join(versionDir, instResDir), ws.InstallationResourceDir)
		require.True(t, file.DirExists(ws.InstallationResourceDir))
	})

}

func cleanup(t *testing.T, versionDir string) {
	//remove relicts from previous runs
	err := os.RemoveAll(versionDir)
	require.NoError(t, err)
}
