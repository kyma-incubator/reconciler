package workspace

import (
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/test"

	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/stretchr/testify/require"
)

const version = "1.20.0"

func TestWorkspaceFactory(t *testing.T) {
	logger := log.NewOptionalLogger(true)

	t.Run("Test validation", func(t *testing.T) {
		wsf1 := Factory{
			logger: logger,
		}
		require.NoError(t, wsf1.validate())
		require.Equal(t, filepath.Join(wsf1.defaultStorageDir(), version), wsf1.workspaceDir(version))
		require.Equal(t, defaultRepositoryURL, wsf1.repository.URL)

		wsf2 := Factory{
			logger:     logger,
			storageDir: "/tmp",
		}
		require.NoError(t, wsf2.validate())
		require.Equal(t, filepath.Join("/tmp", version), wsf2.workspaceDir(version))
		require.Equal(t, defaultRepositoryURL, wsf1.repository.URL)
	})

	t.Run("Clone and delete workspace", func(t *testing.T) {
		test.IntegrationTest(t)

		workspaceDir := filepath.Join(".", "test", version)
		wsf, err := NewFactory(nil, "test", log.NewOptionalLogger(true))
		require.NoError(t, err)

		//cleanup at the beginning (if test was interrupted before)
		testDelete(t, wsf)
		//cleanup at the end (if test finishes regularly)
		defer testDelete(t, wsf)

		ws, err := wsf.Get(version)
		require.NoError(t, err)

		require.Equal(t, filepath.Join(workspaceDir, resDir), ws.ResourceDir)
		require.True(t, file.DirExists(ws.ResourceDir))
		require.Equal(t, filepath.Join(workspaceDir, instResDir), ws.InstallationResourceDir)
		require.True(t, file.DirExists(ws.InstallationResourceDir))
		require.Equal(t, filepath.Join(workspaceDir, instResCrdDir), ws.InstallationResourceCrdDir)
		require.True(t, file.DirExists(ws.InstallationResourceCrdDir))

		//delete success file
		t.Log("Deleting success file to simulate broken workspace")
		err = os.Remove(filepath.Join(workspaceDir, successFile))
		require.NoError(t, err)

		//trigger re-cloning
		ws, err = wsf.Get(version)
		require.NoError(t, err)

		//check again all the required files including success file
		require.True(t, file.DirExists(ws.ResourceDir))
		require.True(t, file.DirExists(ws.InstallationResourceDir))
		require.True(t, file.DirExists(ws.InstallationResourceCrdDir))
		require.True(t, file.Exists(filepath.Join(workspaceDir, successFile)))
	})

}

func testDelete(t *testing.T, wsf *Factory) {
	require.NoError(t, wsf.Delete(version))
}
