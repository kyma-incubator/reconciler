package workspace

import (
	"os"
	"path/filepath"
	"testing"

	log "github.com/kyma-incubator/reconciler/pkg/logger"

	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

const (
	version            = "1.24.0"
	workspaceInHomeDir = "reconciliation-test"
)

func TestWorkspaceFactory(t *testing.T) {
	logger := log.NewLogger(true)

	t.Run("Test validation", func(t *testing.T) {
		wsf1 := DefaultFactory{
			logger: logger,
		}
		require.NoError(t, wsf1.validate())
		require.Equal(t, filepath.Join(wsf1.defaultStorageDir(), version), wsf1.workspaceDir(version))
		require.Equal(t, defaultRepositoryURL, wsf1.repository.URL)

		wsf2 := DefaultFactory{
			logger:     logger,
			storageDir: "/tmp",
		}
		require.NoError(t, wsf2.validate())
		require.Equal(t, filepath.Join("/tmp", version), wsf2.workspaceDir(version))
		require.Equal(t, defaultRepositoryURL, wsf1.repository.URL)
	})

	t.Run("Clone and delete workspace", func(t *testing.T) {
		test.IntegrationTest(t)

		dirname, err := os.UserHomeDir()
		require.NoError(t, err)

		workspaceDir := filepath.Join(dirname, workspaceInHomeDir, version)
		wsf, err := NewFactory(nil, filepath.Join(dirname, workspaceInHomeDir), log.NewLogger(true))
		require.NoError(t, err)

		//cleanup at the beginning (if test was interrupted before)
		testDelete(t, wsf)

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
		err = os.Remove(filepath.Join(workspaceDir, wsReadyIndicatorFile))
		require.NoError(t, err)

		//trigger re-cloning
		ws, err = wsf.Get(version)
		require.NoError(t, err)

		//check again all the required files including success file
		checkWorkspaceDirectories(t, ws)
		require.True(t, file.Exists(filepath.Join(workspaceDir, wsReadyIndicatorFile)))
	})

	t.Run("Use local workspace", func(t *testing.T) {
		workspaceDir := filepath.Join(".", "test", "local")
		wsf, err := NewFactory(&reconciler.Repository{}, workspaceDir, log.NewLogger(true))
		require.NoError(t, err)
		localWs, err := wsf.Get(VersionLocal)
		require.NoError(t, err)
		checkWorkspaceDirectories(t, localWs)
	})
}

func checkWorkspaceDirectories(t *testing.T, ws *Workspace) {
	require.True(t, file.DirExists(ws.ResourceDir))
	require.True(t, file.DirExists(ws.InstallationResourceDir))
	require.True(t, file.DirExists(ws.InstallationResourceCrdDir))
}

func testDelete(t *testing.T, wsf Factory) {
	require.NoError(t, wsf.Delete(version))
}
