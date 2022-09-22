package chart

import (
	"crypto/sha1" //nolint
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	file "github.com/kyma-incubator/reconciler/pkg/files"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

const (
	version = "1.20.0"

	// This is a pre-prepared repo with the following hashes to be used for testing.
	// If the repo is changed the following hashes need to be updated as well.
	upstreamTestRepo  = "https://github.com/moelsayed/nginx-test.git"
	masterHead        = "ef00478f9403d11a3a14203b9219b0ac831b6b18"
	initialCommitHash = "2178444d9a356dd44446e343ab5903247bbe979d"
	mainHead          = "4742250788e8b4c8a50ade9b9b0baa5f13ce4c8d"
	tag               = "v0.1.1"
	tagHash           = "2af1d3a0f2479ea6b46cd38ab61cc74f47f62038"
)

var storageDir = filepath.Join("/tmp", "test", "factory")

// prepares handler func servig archive from given directory
func handlerFuncArchive(t *testing.T, dirname string) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		filename := path.Base(r.URL.Path)
		fullpath := path.Join(dirname, filename)

		_, err := os.Stat(fullpath)
		if err != nil {
			t.Fatalf("unable to serve archive: %q", fullpath)
		}

		http.ServeFile(rw, r, fullpath)
	}
}

func clearWorkspaces(t *testing.T, wss []*Workspace) {
	for _, ws := range wss {
		require.NoError(t, ws.delete())
	}
}

func TestWorkspaceFactory(t *testing.T) {
	logger := log.NewLogger(true)

	t.Run("Test validation", func(t *testing.T) {
		wsf1 := DefaultFactory{
			logger: logger,
		}
		require.NoError(t, wsf1.validate())
		require.Equal(t, filepath.Join(wsf1.defaultStorageDir(), version), wsf1.workspaceDir(version))
		require.Equal(t, defaultRepositoryURL, wsf1.kymaRepository.URL)

		wsf2 := DefaultFactory{
			logger:     logger,
			storageDir: storageDir,
		}
		require.NoError(t, wsf2.validate())
		require.Equal(t, filepath.Join(storageDir, version), wsf2.workspaceDir(version))
		require.Equal(t, defaultRepositoryURL, wsf1.kymaRepository.URL)
	})

	t.Run("Clone and delete workspace", func(t *testing.T) {
		test.IntegrationTest(t)

		dirname, err := os.UserHomeDir()
		require.NoError(t, err)

		workspaceDir := filepath.Join(dirname, workspaceInHomeDir, version)
		wsf, err := NewFactory(nil, filepath.Join(dirname, workspaceInHomeDir), log.NewLogger(true))
		require.NoError(t, err)

		//cleanup at the beginning (if test was interrupted before)
		require.NoError(t, os.RemoveAll(workspaceDir))

		ws, err := wsf.Get(version)
		require.NoError(t, err)
		//cleanup at the end (if test finishes regularly)
		defer testDelete(t, ws.Workspace)

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

	rscdir, err := filepath.Abs("test/unittest-kyma/resources/archives")
	if err != nil {
		t.Fatalf("invalid resource directory: %s", err)
	}

	handler := handlerFuncArchive(t, rscdir)
	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("Create external component from archive", func(t *testing.T) {
		if err := os.MkdirAll(storageDir, 0777); err != nil {
			t.Error(err)
		}

		factory := &DefaultFactory{logger: logger, storageDir: storageDir}

		_, err := os.MkdirTemp(factory.storageDir, "test_*")
		if err != nil {
			t.Error(err)
			return
		}

		fis := assertFileInfos(t, rscdir)
		wss := make([]*Workspace, len(fis))

		for i, fi := range fis {
			version := fmt.Sprintf("version-%d", i)
			ws, name, err := doGetExternalComponent(factory, fi, version, server.URL)
			if err != nil {
				t.Log(err)
				continue
			}

			if err := validateWorkspace(fi.Name(), name, server.URL); err != nil {
				t.Log(err)
				continue
			}

			wss[i] = ws
		}

		defer clearWorkspaces(t, wss)
	})

	t.Run("race-condition", func(t *testing.T) {
		if err := os.MkdirAll(storageDir, 0777); err != nil {
			t.Error(err)
		}

		factory := &DefaultFactory{logger: logger, storageDir: storageDir}

		_, err := ioutil.TempDir(factory.storageDir, "test_*")
		if err != nil {
			t.Error(err)
			return
		}

		fis := assertFileInfos(t, rscdir)

		wg := sync.WaitGroup{}
		var m sync.Mutex
		startAt := time.Now().Add(2 * time.Second)
		max := 10
		wss := make([]*Workspace, 0)

		for i := 1; i <= max; i++ {
			index := i
			wg.Add(1)
			go func(waitUntil time.Time) {
				defer wg.Done()
				time.Sleep(time.Until(waitUntil))

				if index%2 == 0 {
					ws, _, err := doGetExternalComponent(factory, fis[0], "main", server.URL)
					if err != nil {
						t.Log("getComponent:", err)
						return
					}

					m.Lock()
					defer m.Unlock()

					wss = append(wss, ws)
					return
				}

				kws, err := factory.Get("main")
				if err != nil {
					t.Log("get:", err)
					return
				}

				m.Lock()
				defer m.Unlock()

				wss = append(wss, kws.Workspace)

			}(startAt)
		}
		wg.Wait()

		defer clearWorkspaces(t, wss)
	})
}

func doGetExternalComponent(factory Factory, fi os.FileInfo, version, url string) (*Workspace, string, error) {
	index := strings.Index(fi.Name(), ".")

	if index == -1 {
		return nil, "", fmt.Errorf("unable to find file extension: %q", fi.Name())
	}

	name := fi.Name()[:index]

	c := NewComponentBuilder(version, name).
		WithURL(fmt.Sprintf("%s/%s", url, fi.Name())).
		Build()

	ws, err := factory.GetExternalComponent(c)
	return ws, name, err
}

func assertFileInfos(t *testing.T, rscdir string) []os.FileInfo {
	fis, err := ioutil.ReadDir(rscdir)
	if err != nil {
		t.Fatalf("unable to read resurce dir: %s", err)
	}
	if len(fis) == 0 {
		t.Fatalf("no resources found in: %q", rscdir)
	}

	return fis
}

func checkWorkspaceDirectories(t *testing.T, ws *KymaWorkspace) {
	require.True(t, file.DirExists(ws.ResourceDir))
	require.True(t, file.DirExists(ws.InstallationResourceDir))
	require.True(t, file.DirExists(ws.InstallationResourceCrdDir))
}

func testDelete(t *testing.T, ws *Workspace) {
	require.NoError(t, ws.delete())
}

func Test_ExternalGitComponent(t *testing.T) {
	test.IntegrationTest(t)

	logger := log.NewLogger(true)
	factory := &DefaultFactory{logger: logger, storageDir: storageDir}
	_, err := ioutil.TempDir(factory.storageDir, "test_*")
	if err != nil {
		t.Error(err)
		return
	}

	c := &Component{
		version: "master",
		url:     upstreamTestRepo,
		name:    "nginx-test",
	}

	t.Run("Clone version for the first time", func(t *testing.T) {
		expectedWs := factory.workspaceDir(fmt.Sprintf("%s-%s", masterHead[0:8], c.name))
		ws, err := factory.GetExternalComponent(c)
		require.NoError(t, err)
		require.Equal(t, expectedWs, ws.WorkspaceDir)
	})

	// To simulate an updated upstream, we will hard reset the local component cache to an older version.
	componentBaseDir := path.Join(factory.componentBaseDir(c), c.name)
	repo, err := gogit.PlainOpen(componentBaseDir)
	require.NoErrorf(t, err, "failed to open the component base dir repo ")
	w, err := repo.Worktree()
	require.NoErrorf(t, err, "failed to open component base dir worktree")
	err = w.Reset(&gogit.ResetOptions{
		Commit: plumbing.NewHash(initialCommitHash),
		Mode:   gogit.HardReset,
	})
	require.NoErrorf(t, err, "failed to reset component base dir worktree")

	head, err := repo.Head()
	require.NoErrorf(t, err, "failed to get component base dir head")
	require.Equalf(t, head.Hash().String(), initialCommitHash, "incorrect repo head after reset")

	t.Run("Ensure the latest is fetched", func(t *testing.T) {
		expectedWs := factory.workspaceDir(fmt.Sprintf("%s-%s", masterHead[0:8], c.name))
		ws, err := factory.GetExternalComponent(c)
		require.NoError(t, err)
		require.Equal(t, expectedWs, ws.WorkspaceDir)
	})

	t.Run("Change component version branch", func(t *testing.T) {
		c.version = "main"
		expectedWs := factory.workspaceDir(fmt.Sprintf("%s-%s", mainHead[0:8], c.name))
		ws, err := factory.GetExternalComponent(c)
		require.NoError(t, err)
		require.Equal(t, expectedWs, ws.WorkspaceDir)
	})

	t.Run("Change component version to a tag", func(t *testing.T) {
		c.version = tag
		expectedWs := factory.workspaceDir(fmt.Sprintf("%s-%s", tagHash[0:8], c.name))
		ws, err := factory.GetExternalComponent(c)
		require.NoError(t, err)
		require.Equal(t, expectedWs, ws.WorkspaceDir)
	})

	t.Run("Clone component with empty version", func(t *testing.T) {
		c.version = ""
		expectedWs := factory.workspaceDir(fmt.Sprintf("%s-%s", masterHead[0:8], c.name))
		ws, err := factory.GetExternalComponent(c)
		require.NoError(t, err)
		require.Equal(t, expectedWs, ws.WorkspaceDir)
	})
}

func validateWorkspace(fileName, componentName, serverURL string) error {
	fileURL := fmt.Sprintf("%s/%s", serverURL, fileName)
	dirname := fmt.Sprintf("%x-%s", sha1.Sum([]byte(fileURL)), componentName) //nolint
	filepath := path.Join(storageDir, dirname, componentName)

	if _, err := os.Stat(filepath); err != nil {
		return fmt.Errorf("invalid workspace structure %q: %s", filepath, err)
	}

	return nil
}
