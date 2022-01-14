package chart

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	file "github.com/kyma-incubator/reconciler/pkg/files"
)

const (
	resDir        = "resources"
	instResDir    = "installation/resources"
	instResCrdDir = "installation/resources/crds"
)

type Workspace struct {
	WorkspaceDir string
}

func (ws *Workspace) delete() error {
	return os.RemoveAll(ws.WorkspaceDir)
}

func newWorkspace(workspaceDir string, validators ...func(*Workspace) error) (*Workspace, error) {
	ws := &Workspace{
		WorkspaceDir: workspaceDir,
	}

	for _, v := range validators {
		if err := v(ws); err != nil {
			return nil, err
		}
	}

	return ws, nil
}

var validateDir = func(w *Workspace) error {
	if !file.DirExists(w.WorkspaceDir) {
		return fmt.Errorf("workspace directory '%s' does not exit", w.WorkspaceDir)
	}
	return nil
}

func containsFile(filename string) func(*Workspace) error {
	return func(w *Workspace) error {
		found := false
		err := filepath.WalkDir(w.WorkspaceDir, func(path string, d fs.DirEntry, err error) error {
			if filepath.Base(path) == filename {
				found = true
			}
			return nil
		})
		if !found {
			return fmt.Errorf("Failed to find %v in %v", filename, w.WorkspaceDir)
		}
		if err != nil {
			return err
		}
		return nil
	}
}

func newComponentWorkspace(workspaceDir string) (*Workspace, error) {
	validateChart := containsFile("Chart.yaml")
	return newWorkspace(workspaceDir, validateDir, validateChart)
}

type KymaWorkspace struct {
	*Workspace
	ResourceDir                string
	InstallationResourceDir    string
	InstallationResourceCrdDir string
}

func newKymaWorkspace(workspaceDir string) (*KymaWorkspace, error) {
	ws, err := newWorkspace(workspaceDir, validateDir)
	if err != nil {
		return nil, err
	}

	kymaWs := &KymaWorkspace{
		ws,
		filepath.Join(workspaceDir, resDir),
		filepath.Join(workspaceDir, instResDir),
		filepath.Join(workspaceDir, instResCrdDir),
	}
	return kymaWs, kymaWs.validate()
}

func (w *KymaWorkspace) validate() error {
	//ensure expected files exist
	for _, dir := range []string{w.ResourceDir, w.InstallationResourceDir, w.InstallationResourceCrdDir} {
		if !file.DirExists(dir) {
			return fmt.Errorf("required resource directory '%s' is missing in Kyma workspace '%s'",
				dir, w.WorkspaceDir)
		}
	}
	return nil
}
