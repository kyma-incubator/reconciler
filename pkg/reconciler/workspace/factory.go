package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/git"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	file "github.com/kyma-incubator/reconciler/pkg/files"
)

const (
	VersionLocal         = "local"
	defaultRepositoryURL = "https://github.com/kyma-project/kyma"
	wsReadyIndicatorFile = "workspace-ready.yaml"
)

//go:generate mockery --name=Factory --outpkg=mock --case=underscore
// Factory of workspace.
type Factory interface {
	// Get workspace of the given version.
	Get(version string) (*Workspace, error)

	// Delete workspace of the given version.
	Delete(version string) error
}

type DefaultFactory struct {
	storageDir string
	logger     *zap.SugaredLogger
	mutex      sync.Mutex
	repository *reconciler.Repository
}

func NewFactory(repo *reconciler.Repository, storageDir string, logger *zap.SugaredLogger) (*DefaultFactory, error) {
	factory := &DefaultFactory{
		storageDir: storageDir,
		logger:     logger,
		repository: repo,
	}
	return factory, factory.validate()
}

func (f *DefaultFactory) String() string {
	return fmt.Sprintf("WorkspaceFactory [storageDir=%s]", f.storageDir)
}

func (f *DefaultFactory) validate() error {
	if f.logger == nil {
		return fmt.Errorf("no logger provided: please set field Logger")
	}
	if f.storageDir == "" {
		f.storageDir = f.defaultStorageDir()
	}
	if f.repository == nil || f.repository.URL == "" {
		f.repository = &reconciler.Repository{
			URL: defaultRepositoryURL,
		}
	}
	return nil
}

func (f *DefaultFactory) workspaceDir(version string) string {
	return filepath.Join(f.storageDir, version) //add Kyma version as subdirectory
}

func (f *DefaultFactory) defaultStorageDir() string {
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

func (f *DefaultFactory) Get(version string) (*Workspace, error) {
	if err := f.validate(); err != nil {
		return nil, err
	}

	if version == VersionLocal {
		//storage should be used as workspace - no cloning required
		return newWorkspace(f.storageDir)
	}

	wsDir := f.workspaceDir(version)

	wsReadyFile := filepath.Join(wsDir, wsReadyIndicatorFile)
	//ensure Kyma sources are available
	if !file.Exists(wsReadyFile) {
		if err := f.clone(version, wsDir); err != nil {
			return nil, err
		}
	}

	return newWorkspace(wsDir)
}

func (f *DefaultFactory) clone(version, dstDir string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	wsReadyFile := filepath.Join(dstDir, wsReadyIndicatorFile)
	if file.Exists(wsReadyFile) {
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
	f.logger.Infof("Cloning repository '%s' with revision '%s' into workspace '%s'",
		f.repository.URL, version, dstDir)
	clientSet, err := kubeclient.NewInClusterClientSet(f.logger)
	if err != nil {
		return err
	}
	cloner, _ := git.NewCloner(&git.Client{}, f.repository, true, clientSet)

	if err := cloner.CloneAndCheckout(dstDir, version); err != nil {
		f.logger.Warnf("Deleting workspace '%s' because GIT clone of repository-URL '%s' with revision '%s' failed",
			dstDir, f.repository.URL, version)
		if removeErr := f.Delete(version); removeErr != nil {
			err = errors.Wrap(err, removeErr.Error())
		}
		return err
	}

	//create a marker file to flag success
	fileHandler, err := os.Create(wsReadyFile)
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

func (f *DefaultFactory) Delete(version string) error {
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
