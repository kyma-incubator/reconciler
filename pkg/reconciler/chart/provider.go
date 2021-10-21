package chart

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"io/ioutil"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

const kindCRD = "CustomResourceDefinition"

//go:generate mockery --name=Provider --outpkg=mock --case=underscore
// Provider of manifests.
type Provider interface {
	// RenderCRD of the given version.
	RenderCRD(version string) ([]*Manifest, error)

	// RenderManifest of the given component.
	RenderManifest(component *Component) (*Manifest, error)

	// Configuration of the given component.
	Configuration(component *Component) (map[string]interface{}, error)
}

// DefaultProvider provides a default implementation of Provider.
type DefaultProvider struct {
	wsFactory workspace.Factory
	logger    *zap.SugaredLogger
}

// NewDefaultProvider returns a new instance of DefaultProvider.
func NewDefaultProvider(wsFactory workspace.Factory, logger *zap.SugaredLogger) (*DefaultProvider, error) {
	if wsFactory == nil {
		return nil, fmt.Errorf("workspace factory cannot be nil")
	}
	return &DefaultProvider{
		wsFactory: wsFactory,
		logger:    logger,
	}, nil
}

func (p *DefaultProvider) RenderCRD(version string) ([]*Manifest, error) {
	ws, err := p.newWorkspace(version, "", "")
	if err != nil {
		return nil, err
	}

	p.logger.Debugf("Rendering CRD resources of Kyma version '%s'", version)

	var manifests []*Manifest
	err = filepath.Walk(ws.InstallationResourceCrdDir,
		func(path string, file os.FileInfo, e error) error {
			if e != nil {
				return e
			}

			if file.IsDir() {
				return nil
			}

			fileExt := filepath.Ext(file.Name())
			if fileExt != ".yaml" && fileExt != ".yml" {
				p.logger.Debugf("Found file in CRD directory with non-supported "+
					"file extension '%s': ignoring it", path)
				return nil
			}

			crdData, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			unstructs, err := kubeclient.ToUnstructured(crdData, true)
			if err != nil {
				return err
			}

			for _, unstruct := range unstructs {
				if unstruct.GetKind() != kindCRD {
					p.logger.Warnf("Found in CRD directory the file '%s' which includes a resource of kind '%s': "+
						"this resource will be ignored", path, unstruct.GetKind())
					continue
				}
				manifests = append(manifests, &Manifest{
					Type:     CRD,
					Name:     path,
					Manifest: string(crdData),
				})
			}

			return nil
		})

	p.logger.Debugf("Found %d CRD resources in Kyma version '%s'", len(manifests), version)

	return manifests, err
}

func (p *DefaultProvider) RenderManifest(component *Component) (*Manifest, error) {
	ws, err := p.newWorkspace(component.version, component.repositoryURL, component.name)
	if err != nil {
		return nil, err
	}

	helmClient, err := NewHelmClient(ws.ResourceDir, p.logger)
	if err != nil {
		return nil, err
	}

	manifest, err := helmClient.Render(component)
	if err != nil {
		return nil, err
	}

	return &Manifest{
		Type:     HelmChart,
		Name:     component.name,
		Manifest: manifest,
	}, nil
}

func (p *DefaultProvider) Configuration(component *Component) (map[string]interface{}, error) {
	ws, err := p.newWorkspace(component.version, component.repositoryURL, component.name)
	if err != nil {
		return nil, err
	}

	helmClient, err := NewHelmClient(ws.ResourceDir, p.logger)
	if err != nil {
		return nil, err
	}

	return helmClient.Configuration(component)
}

func (p *DefaultProvider) newWorkspace(version, repository, componentName string) (*workspace.Workspace, error) {
	p.logger.Debugf("Getting workspace for Kyma '%s', repository: '%s'", version, repository)
	ws, err := p.wsFactory.Get(version, repository, componentName)
	if err != nil {
		p.logger.Warnf("Failed to retrieve workspace for Kyma '%s', repository: '%s': %s", version, repository, err)
	}
	return ws, err
}
