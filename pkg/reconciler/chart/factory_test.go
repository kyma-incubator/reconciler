package chart

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	file "github.com/kyma-incubator/reconciler/pkg/files"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

const (
	version = "1.20.0"
)

var storageDir = filepath.Join("test", "factory")

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

func clearWorkspaces(t *testing.T, f *DefaultFactory, vs []string) {
	for _, v := range vs {
		if err := f.Delete(v); err != nil {
			t.Logf("unable to remove version: %q in %q", v, f.storageDir)
		}
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
		factory := &DefaultFactory{logger: logger, storageDir: storageDir}

		_, err := ioutil.TempDir(factory.storageDir, "test_*")
		if err != nil {
			t.Error(err)
			return
		}

		fis := assertFileInfos(t, rscdir)
		vds := make([]string, len(fis))

		for i, fi := range fis {
			version := fmt.Sprintf("version-%d", i)
			if err := doGetExternalComponent(factory, fi, version, server.URL); err != nil {
				t.Log(err)
				continue
			}
			vds[i] = version
		}

		defer clearWorkspaces(t, factory, vds)
	})

	t.Run("race-condition", func(t *testing.T) {
		factory := &DefaultFactory{logger: logger, storageDir: storageDir}

		_, err := ioutil.TempDir(factory.storageDir, "test_*")
		if err != nil {
			t.Error(err)
			return
		}

		fis := assertFileInfos(t, rscdir)

		wg := sync.WaitGroup{}
		startAt := time.Now().Add(2 * time.Second)
		max := 10

		for i := 1; i <= max; i++ {
			index := i
			wg.Add(1)
			go func(waitUntil time.Time) {
				defer wg.Done()
				time.Sleep(time.Until(waitUntil))

				if index%2 == 0 {
					err := doGetExternalComponent(factory, fis[0], "main", server.URL)
					if err != nil {
						t.Log("getComponent:", err)
						return
					}
				}

				_, err := factory.Get("main")
				if err != nil {
					t.Log("get:", err)
				}
			}(startAt)
		}
		wg.Wait()

		defer clearWorkspaces(t, factory, []string{"race-condition-external", "race-condition-kyma", "main"})
	})
}

func doGetExternalComponent(factory Factory, fi os.FileInfo, version, url string) error {
	index := strings.Index(fi.Name(), ".")

	if index == -1 {
		return fmt.Errorf("unable to find file extension: %q", fi.Name())
	}

	name := fi.Name()[:index]

	c := NewComponentBuilder(version, name).
		WithURL(fmt.Sprintf("%s/%s", url, fi.Name())).
		Build()

	_, err := factory.GetExternalComponent(c)
	return err
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

func testDelete(t *testing.T, wsf Factory) {
	require.NoError(t, wsf.Delete(version))
}
