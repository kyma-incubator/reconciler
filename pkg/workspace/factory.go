package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/kyma-incubator/reconciler/pkg/git"
	"github.com/pkg/errors"
)

const (
	defaultRepositoryURL = "https://github.com/kyma-project/kyma"
	resDir               = "resources"
	instResDir           = "installation/resources"
	componentFile        = "installation/resources/components.yaml"
)

type Workspace struct {
	ComponentFile           string
	ResourceDir             string
	InstallationResourceDir string
}

type Factory struct {
	mutex         sync.Mutex
	StorageDir    string
	RepositoryURL string
}

func (f *Factory) defaultStorageDir() string {
	//define work dir, priority: "$HOME", "cwd()", "."
	baseDir, err := os.UserHomeDir()
	if err != nil {
		baseDir, err = os.Getwd()
		if err != nil {
			baseDir = "."
		}
	}
	return filepath.Join(baseDir, ".kyma", "reconciler", "versions")
}

func (f *Factory) versionDir(version string) string {
	if f.StorageDir == "" {
		f.StorageDir = f.defaultStorageDir()
	}

	versionDir := filepath.Join(f.StorageDir, version) //add Kyma version as subdirectory

	return versionDir
}

func (f *Factory) Get(version string) (*Workspace, error) {
	versionDir := f.versionDir(version)

	//ensure Kyma sources are available
	if !file.DirExists(versionDir) {
		if err := f.clone(version, versionDir); err != nil {
			return nil, err
		}
	}

	//return workspace
	return &Workspace{
		ComponentFile:           filepath.Join(versionDir, componentFile),
		ResourceDir:             filepath.Join(versionDir, resDir),
		InstallationResourceDir: filepath.Join(versionDir, instResDir),
	}, nil
}

func (f *Factory) clone(version, dstDir string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if file.DirExists(dstDir) {
		//race condition protection: it could happen that a previous go-routing was also triggering the clone of the Kyma version
		return nil
	}

	repoURL := f.RepositoryURL
	if repoURL == "" {
		repoURL = defaultRepositoryURL
	}

	//clone sources
	if err := git.CloneRepo(repoURL, dstDir, version); err != nil {
		if removeErr := os.RemoveAll(dstDir); removeErr != nil { //git failed, cleanup incomplete clones
			return errors.Wrap(err, removeErr.Error())
		}
		return err
	}
	//ensure expected files exist
	for _, dir := range []string{resDir, instResDir} {
		reqDir := filepath.Join(dstDir, dir)
		if !file.DirExists(reqDir) {
			return fmt.Errorf("Required resource directory '%s' is missing in Kyma version '%s'", reqDir, version)
		}
	}
	reqFile := filepath.Join(dstDir, componentFile)
	if !file.Exists(reqFile) {
		return fmt.Errorf("Required component file '%s' is missing in Kyma version '%s'", reqFile, version)
	}

	//clone ready for use
	return nil
}
