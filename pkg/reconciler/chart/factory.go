package chart

import (
	"crypto/sha1" //nolint
	"fmt"
	"io"
	"net/http"
	"path"

	"os"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/mholt/archiver/v3"
	"github.com/otiai10/copy"

	"path/filepath"
	"sync"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/git"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	file "github.com/kyma-incubator/reconciler/pkg/files"
)

const (
	VersionLocal         = "local"
	defaultRepositoryURL = "https://github.com/kyma-project/kyma"
	wsReadyIndicatorFile = "workspace-ready.yaml"

	gitComponentsBaseDir = "base"
)

// Factory of workspace.
//
//go:generate mockery --name=Factory --outpkg=mocks --case=underscore
type Factory interface {
	// Get workspace of the given Kyma version.
	Get(version string) (*KymaWorkspace, error)
	// Delete workspace of the given Kyma version.
	Delete(version string) error

	GetExternalComponent(component *Component) (*Workspace, error)
}

type DefaultFactory struct {
	storageDir        string
	logger            *zap.SugaredLogger
	mutexGet          sync.Mutex
	mutexGetComponent sync.Mutex
	kymaRepository    *reconciler.Repository
}

func NewFactory(repo *reconciler.Repository, storageDir string, logger *zap.SugaredLogger) (*DefaultFactory, error) {
	factory := &DefaultFactory{
		storageDir:     storageDir,
		logger:         logger,
		kymaRepository: repo,
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
	if f.kymaRepository == nil || f.kymaRepository.URL == "" {
		f.kymaRepository = &reconciler.Repository{
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
	return filepath.Join(baseDir, ".kyma", "reconciler", "workspaces")
}

func (f *DefaultFactory) Get(version string) (*KymaWorkspace, error) {
	f.mutexGet.Lock()
	defer f.mutexGet.Unlock()

	if err := f.validate(); err != nil {
		return nil, err
	}

	if version == VersionLocal {
		//storage should be used as workspace - no cloning required
		return newKymaWorkspace(f.storageDir)
	}

	wsDir := f.workspaceDir(version)

	wsReadyFile := filepath.Join(wsDir, wsReadyIndicatorFile)
	if file.Exists(wsReadyFile) {
		f.logger.Debugf("Workspace '%s' already exists", wsDir)
		return newKymaWorkspace(wsDir)
	}

	if file.DirExists(wsDir) {
		f.logger.Warnf("Deleting workspace '%s' because previous download does not contain all the required files", wsDir)
		if err := os.RemoveAll(wsDir); err != nil {
			return nil, err
		}
	}

	if err := f.clone(version, wsDir, wsDir, f.kymaRepository); err != nil {
		return nil, err
	}

	return newKymaWorkspace(wsDir)
}

func (f *DefaultFactory) GetExternalComponent(component *Component) (*Workspace, error) {
	f.mutexGetComponent.Lock()
	defer f.mutexGetComponent.Unlock()

	if component == nil {
		return nil, errors.New("cannot retrieve workspace because provided component was 'nil'")
	}

	if component.isExternalGitComponent() {
		return f.getExternalGitComponent(component)
	}

	return f.getExternalArchiveComponent(component)
}

func (f *DefaultFactory) getExternalArchiveComponent(component *Component) (*Workspace, error) {
	wsDir := f.componentBaseDir(component)

	if f.readyMarkerExists(wsDir) {
		return newComponentWorkspace(wsDir)
	}

	if err := f.cleanFailedWorkspace(wsDir); err != nil {
		return nil, err
	}

	f.logger.Infof("Downloading component '%s' with version '%s' from source '%s' into workspace '%s'",
		component.name, component.version, component.url, wsDir)

	if err := f.downloadComponent(component, wsDir); err != nil {
		return nil, err
	}

	return newComponentWorkspace(wsDir)
}

func (f *DefaultFactory) getExternalGitComponent(component *Component) (*Workspace, error) {
	baseDir := f.componentBaseDir(component)

	if f.readyMarkerExists(baseDir) { // already cloned, just fetch
		if err := f.fetchComponent(component, baseDir); err != nil {
			return nil, err
		}
	} else {
		if err := f.cleanFailedWorkspace(baseDir); err != nil {
			return nil, err
		}
		if err := f.cloneComponent(component, baseDir); err != nil {
			return nil, err
		}
	}

	wsDir, err := f.copyComponentLatestRev(component, baseDir)
	if err != nil {
		return nil, err
	}
	return newComponentWorkspace(wsDir)
}

func (f *DefaultFactory) cloneComponent(component *Component, dstDir string) error {
	f.logger.Infof("Cloning component '%s' with version '%s' from source '%s' into workspace '%s'",
		component.name, component.version, component.url, dstDir)

	repo := &reconciler.Repository{
		URL: component.url,
	}

	dstPath := path.Join(dstDir, component.name)
	return f.clone(component.version, dstPath, dstDir, repo)
}

func (f *DefaultFactory) downloadComponent(component *Component, dstDir string) error {
	// create dst dir
	if err := os.MkdirAll(dstDir, 0700); err != nil {
		f.logger.Warnf("Unable to create destination directory: %q", dstDir)
	}

	// TODO consider extracting file to memory
	tmpFile, err := f.downloadArchive(component.url, dstDir, component.isInternalGitComponent())
	if err != nil {
		return err
	}
	defer func() {
		// delete downloaded file after unarchiving it
		if err := os.Remove(tmpFile); err != nil {
			f.logger.Warnf("Unable to remove archive file %q: %s", tmpFile, err)
		}
	}()

	err = archiver.Unarchive(tmpFile, dstDir)
	if err != nil {
		return err
	}

	//create a marker file to flag success
	if err := f.createReadyMarker(dstDir); err != nil {
		return err
	}

	return nil
}

func (f *DefaultFactory) downloadArchive(URL, dstDir string, useToken bool) (string, error) {

	req, err := http.NewRequest("GET", URL, nil)

	if useToken {
		f.logger.Infof("Downloading archive '%s' into workspace '%s' from private repo", URL, dstDir)

		token := os.Getenv("GIT_CLONE_TOKEN")
		if token == "" {
			return "", errors.New("token value not found")
		}

		var bearer = "Bearer " + token
		req.Header.Add("Authorization", bearer)
	} else {
		f.logger.Infof("Downloading archive '%s' into workspace '%s' from public repo", URL, dstDir)
	}

	client := &http.Client{}
	resp, err := client.Do(req) // #nosec
	if err != nil {
		return "", err
	}
	//} else {
	//
	//	resp, err := http.Get(URL) // #nosec
	//	if err != nil {
	//		return "", err
	//	}
	//}

	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return "", fmt.Errorf("not found: %q", URL)
	}

	b := make([]byte, 255)
	if _, err := resp.Body.Read(b); err != nil {
		return "", err
	}

	mimeType := http.DetectContentType(b)
	// the extension is required by the archiver
	extension, err := extension(mimeType)
	if err != nil {
		return "", err
	}

	filenameTpl := fmt.Sprintf("component_*.%s", extension)
	tmpFile, err := os.CreateTemp(dstDir, filenameTpl)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := tmpFile.Close(); err != nil {
			f.logger.Warnf("Failed to close file handler for tmp-file '%s': %s", tmpFile.Name(), err)
		}
	}()

	// first write bytes used to get the mime type
	_, err = tmpFile.Write(b)
	if err != nil {
		return "", err
	}
	// write the rest of the archive
	_, err = io.Copy(tmpFile, resp.Body)

	return tmpFile.Name(), err
}

func extension(mimeType string) (string, error) {
	switch mimeType {
	case "application/x-gzip":
		return "tar.gz", nil
	case "application/zip":
		return "zip", nil
	case "application/x-rar-compressed":
		return "rar", nil
	default:
		return "", fmt.Errorf("unsupported archive")
	}
}

func (f *DefaultFactory) readyFile(dstDir string) string {
	return filepath.Join(dstDir, wsReadyIndicatorFile)
}

func (f *DefaultFactory) clone(version string, dstDir string, markerDir string, repo *reconciler.Repository) error {
	f.logger.Infof("Cloning GIT repository '%s' with revision '%s' into workspace '%s'",
		repo.URL, version, dstDir)

	cloner, _ := git.NewCloner(&git.Client{}, repo, true, f.logger)
	if err := cloner.CloneAndCheckout(dstDir, version); err != nil {
		f.logger.Warnf("Deleting workspace '%s' because GIT clone of repository-URL '%s' with revision '%s' failed",
			dstDir, repo.URL, version)

		if removeErr := os.RemoveAll(dstDir); removeErr != nil {
			err = errors.Wrap(err, removeErr.Error())
		}
		return err
	}
	//create a marker file to flag success
	if err := f.createReadyMarker(markerDir); err != nil {
		return err
	}
	return nil
}

func (f *DefaultFactory) Delete(version string) error {
	if err := f.validate(); err != nil {
		return err
	}
	wsDir := f.workspaceDir(version)
	f.logger.Infof("Deleting workspace '%s'", wsDir)
	err := os.RemoveAll(wsDir)
	if err != nil {
		f.logger.Warnf("Failed to delete workspace '%s': %s", wsDir, err)
	}
	return err
}

func (f *DefaultFactory) componentBaseDir(c *Component) string {
	filename := GetExternalArchiveComponentHashedVersion(c.url, c.name)

	if c.isExternalGitComponent() {
		return filepath.Join(f.storageDir, gitComponentsBaseDir, filename)
	}

	return filepath.Join(f.storageDir, filename)
}

func GetExternalArchiveComponentHashedVersion(url, name string) string {
	return fmt.Sprintf("%x-%s", sha1.Sum([]byte(url)), name) //nolint
}

func (f *DefaultFactory) readyMarkerExists(baseDir string) bool {
	return file.Exists(filepath.Join(baseDir, wsReadyIndicatorFile))
}

func (f *DefaultFactory) cleanFailedWorkspace(baseDir string) error {
	if !file.DirExists(baseDir) {
		return nil
	}
	f.logger.Warnf("Deleting workspace '%s' because previous download/clone does not contain all the required files", baseDir)
	return os.RemoveAll(baseDir)
}

func (f *DefaultFactory) fetchComponent(component *Component, dstDir string) error {
	dstPath := path.Join(dstDir, component.name)
	f.logger.Infof("Fetching GIT repository '%s' in workspace '%s'",
		component.url, dstPath)

	repo := &reconciler.Repository{
		URL: component.url,
	}

	cloner, _ := git.NewCloner(&git.Client{}, repo, true, f.logger)
	return cloner.FetchAndCheckout(dstPath, component.version)
}

// getLatestRevOfVersion works on local cache. If "version" is a branch, it will retrun the revisionID of it's HEAD
// if "version" is a tag, it will return it's revisionID.
func (f *DefaultFactory) getLatestRevOfVersion(version, path string) (string, error) {
	gitClient, err := git.NewClientWithPath(path)
	if err != nil {
		return "", err
	}

	cloner, _ := git.NewCloner(gitClient, nil, true, f.logger)
	revision, err := cloner.ResolveRevisionOrBranchHead(version)
	if err != nil {
		return "", err
	}

	return revision.String(), nil
}

func (f *DefaultFactory) copyComponentLatestRev(component *Component, baseDir string) (string, error) {
	rev, err := f.getLatestRevOfVersion(component.version, path.Join(baseDir, component.name))
	if err != nil {
		return "", err
	}
	wsDir := f.workspaceDir(fmt.Sprintf("%s-%s", rev[0:8], component.name))

	if f.readyMarkerExists(wsDir) {
		return wsDir, nil
	}
	if err := f.cleanFailedWorkspace(wsDir); err != nil {
		return "", err
	}
	destWsDir := path.Join(wsDir, component.name)
	componentBaseDir := path.Join(baseDir, component.name)

	if err = copy.Copy(componentBaseDir, destWsDir); err != nil {
		return "", err
	}
	gitClient, err := git.NewClientWithPath(destWsDir)
	if err != nil {
		return "", err
	}
	if err := gitClient.PlainCheckout(&gogit.CheckoutOptions{
		Hash: plumbing.NewHash(rev),
	}); err != nil {
		return "", err
	}
	if err := f.createReadyMarker(wsDir); err != nil {
		return "", err
	}

	return wsDir, nil
}

func (f *DefaultFactory) createReadyMarker(wsDir string) error {
	fileHandler, err := os.Create(f.readyFile(wsDir))
	if err != nil {
		return err
	}

	if err := fileHandler.Close(); err != nil {
		f.logger.Warnf("Failed to close marker file: %s", err)
	}
	return nil
}
