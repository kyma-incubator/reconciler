package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/git"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	file "github.com/kyma-incubator/reconciler/pkg/files"
)

const (
	defaultRepositoryURL = "https://github.com/kyma-project/kyma"
	resDir               = "resources"
	instResDir           = "installation/resources"
	componentFile        = "installation/resources/components.yaml"
	successFile          = "success.yaml"
)

type Workspace struct {
	ComponentFile           string
	ResourceDir             string
	InstallationResourceDir string
}

type Factory struct {
	Debug         bool
	StorageDir    string
	RepositoryURL string
	mutex         sync.Mutex
	logger        *zap.SugaredLogger
	workspaceDir  string
}

func (f *Factory) validate(version string) error {
	var err error
	f.logger, err = logger.NewLogger(f.Debug)
	if err != nil {
		return err
	}
	if f.StorageDir == "" {
		f.StorageDir = f.defaultStorageDir()
	}
	if f.RepositoryURL == "" {
		f.RepositoryURL = defaultRepositoryURL
	}
	f.workspaceDir = filepath.Join(f.StorageDir, version) //add Kyma version as subdirectory
	return nil
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

func (f *Factory) Get(version string) (*Workspace, error) {
	if err := f.validate(version); err != nil {
		return nil, err
	}

	sFile := filepath.Join(f.workspaceDir, successFile)
	//ensure Kyma sources are available
	if !file.Exists(sFile) {
		f.logger.Infof("Creating new workspace directory '%s' ", f.workspaceDir)
		if err := f.clone(version, f.workspaceDir); err != nil {
			return nil, err
		}
	}

	//return workspace
	return &Workspace{
		ComponentFile:           filepath.Join(f.workspaceDir, componentFile),
		ResourceDir:             filepath.Join(f.workspaceDir, resDir),
		InstallationResourceDir: filepath.Join(f.workspaceDir, instResDir),
	}, nil
}

func (f *Factory) clone(version, dstDir string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	sFile := filepath.Join(dstDir, successFile)
	if file.Exists(sFile) {
		f.logger.Debugf("Workspace '%s' already exists", dstDir)
		//race condition protection: it could happen that a previous go-routing was also triggering the clone of the Kyma version
		return nil
	}
	if file.DirExists(dstDir) {
		//if workspace exists but there is no success file, it is probably corrupted, so delete it
		f.logger.Warnf("Deleting workspace '%s' because GIT clone does not contain all the required files", dstDir)
		if err := os.RemoveAll(dstDir); err != nil {
			return err
		}
	}

	//clone sources
	f.logger.Infof("Start cloning repository '%s' with revision '%s' into workspace '%s'",
		f.RepositoryURL, version, dstDir)
	if err := git.CloneRepo(f.RepositoryURL, dstDir, version); err != nil {
		f.logger.Warnf("Deleting workspace '%s' because GIT clone of repository-URL '%s' with revision '%s' failed",
			dstDir, f.RepositoryURL, version)
		if removeErr := f.Delete(version); removeErr != nil {
			err = errors.Wrap(err, removeErr.Error())
		}
		return err
	}
	//ensure expected files exist
	for _, dir := range []string{resDir, instResDir} {
		reqDir := filepath.Join(dstDir, dir)
		if !file.DirExists(reqDir) {
			return fmt.Errorf("required resource directory '%s' is missing in Kyma version '%s'", reqDir, version)
		}
	}
	reqFile := filepath.Join(dstDir, componentFile)
	if !file.Exists(reqFile) {
		return fmt.Errorf("required component file '%s' is missing in Kyma version '%s'", reqFile, version)
	}

	//create a marker file to flag success
	fileHandler, err := os.Create(sFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := fileHandler.Close(); err != nil {
			f.logger.Warnf("Failed to close marker file: %s", err)
		}
	}()

	//clone ready for use
	return nil
}

func (f *Factory) Delete(version string) error {
	if err := f.validate(version); err != nil {
		return err
	}
	f.logger.Debugf("Deleting workspace '%s'", f.workspaceDir)
	err := os.RemoveAll(f.workspaceDir)
	if err != nil {
		f.logger.Warnf("Failed to delete workspace '%s': %s", f.workspaceDir, err)
	}
	return err
}
