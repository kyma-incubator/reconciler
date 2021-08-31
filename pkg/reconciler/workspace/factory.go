package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/git"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	file "github.com/kyma-incubator/reconciler/pkg/files"
)

const (
	defaultRepositoryURL = "https://github.com/kyma-project/kyma"
	resDir               = "resources"
	instResDir           = "installation/resources"
	instResCrdDir        = "installation/resources/crds"
	successFile          = "success.yaml"
)

type Workspace struct {
	WorkspaceDir               string
	ResourceDir                string
	InstallationResourceDir    string
	InstallationResourceCrdDir string
}

type Factory struct {
	storageDir    string
	repositoryURL string
	logger        *zap.SugaredLogger
	mutex         sync.Mutex
}

func NewFactory(storageDir string, logger *zap.SugaredLogger) (*Factory, error) {
	factory := &Factory{
		storageDir:    storageDir,
		logger:        logger,
		repositoryURL: defaultRepositoryURL,
	}
	return factory, factory.validate()
}

func (f *Factory) String() string {
	return fmt.Sprintf("WorkspaceFactory [storageDir=%s]", f.storageDir)
}

func (f *Factory) validate() error {
	if f.logger == nil {
		return fmt.Errorf("no logger provided: please set field Logger")
	}
	if f.storageDir == "" {
		f.storageDir = f.defaultStorageDir()
	}
	return nil
}

func (f *Factory) workspaceDir(version string) string {
	return filepath.Join(f.storageDir, version) //add Kyma version as subdirectory
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
	if err := f.validate(); err != nil {
		return nil, err
	}

	wsDir := f.workspaceDir(version)

	sFile := filepath.Join(wsDir, successFile)
	//ensure Kyma sources are available
	if !file.Exists(sFile) {
		if err := f.clone(version, wsDir); err != nil {
			return nil, err
		}
	}

	//return workspace
	return &Workspace{
		WorkspaceDir:               wsDir,
		ResourceDir:                filepath.Join(wsDir, resDir),
		InstallationResourceDir:    filepath.Join(wsDir, instResDir),
		InstallationResourceCrdDir: filepath.Join(wsDir, instResCrdDir),
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
	f.logger.Infof("Cloning repository '%s' with revision '%s' into workspace directory '%s'",
		f.repositoryURL, version, dstDir)
	if err := git.CloneRepo(f.repositoryURL, dstDir, version); err != nil {
		f.logger.Warnf("Deleting workspace '%s' because GIT clone of repository-URL '%s' with revision '%s' failed",
			dstDir, f.repositoryURL, version)
		if removeErr := f.Delete(version); removeErr != nil {
			err = errors.Wrap(err, removeErr.Error())
		}
		return err
	}
	//ensure expected files exist
	for _, dir := range []string{resDir, instResDir, instResCrdDir} {
		reqDir := filepath.Join(dstDir, dir)
		if !file.DirExists(reqDir) {
			return fmt.Errorf("required resource directory '%s' is missing in Kyma version '%s'", reqDir, version)
		}
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
	if err := f.validate(); err != nil {
		return err
	}
	wsDir := f.workspaceDir(version)
	f.logger.Debugf("Deleting workspace '%s'", wsDir)
	err := os.RemoveAll(wsDir)
	if err != nil {
		f.logger.Warnf("Failed to delete workspace '%s': %s", wsDir, err)
	}
	return err
}
