package workspace

import (
	"fmt"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"path/filepath"
)

const (
	resDir        = "resources"
	instResDir    = "installation/resources"
	instResCrdDir = "installation/resources/crds"
)

type Workspace struct {
	WorkspaceDir               string
	ResourceDir                string
	InstallationResourceDir    string
	InstallationResourceCrdDir string
}

func newWorkspace(workspaceDir string) (*Workspace, error) {
	ws := &Workspace{
		WorkspaceDir:               workspaceDir,
		ResourceDir:                filepath.Join(workspaceDir, resDir),
		InstallationResourceDir:    filepath.Join(workspaceDir, instResDir),
		InstallationResourceCrdDir: filepath.Join(workspaceDir, instResCrdDir),
	}
	return ws, ws.validate()
}

func (w *Workspace) validate() error {
	//ensure expected files exist
	for _, dir := range []string{w.ResourceDir, w.InstallationResourceDir, w.InstallationResourceCrdDir} {
		if !file.DirExists(dir) {
			return fmt.Errorf("required resource directory '%s' is missing in Kyma workspace '%s'",
				dir, w.WorkspaceDir)
		}
	}
	return nil
}
