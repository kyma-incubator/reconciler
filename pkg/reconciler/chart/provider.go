package chart

import (
	"fmt"
	"os"
	"path/filepath"

	fileUtils "github.com/kyma-incubator/reconciler/pkg/files"
	reconcilerK8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"

	"go.uber.org/zap"
)

const kindCRD = "CustomResourceDefinition"

//go:generate mockery --name=Provider --outpkg=mocks --case=underscore
// Provider of manifests.
type Provider interface {
	// WithFilter adds manifest filter to the Provider's filters
	WithFilter(filter Filter) Provider

	// RenderCRD of the given version.
	RenderCRD(version string) ([]*Manifest, error)

	// RenderManifest of the given component.
	RenderManifest(component *Component) (*Manifest, error)

	// Configuration of the given component.
	Configuration(component *Component) (map[string]interface{}, error)
}

type Filter func(string) (string, error)

// DefaultProvider provides a default implementation of Provider.
type DefaultProvider struct {
	wsFactory Factory
	logger    *zap.SugaredLogger
	filters   []Filter
}

// NewDefaultProvider returns a new instance of DefaultProvider.
func NewDefaultProvider(wsFactory Factory, logger *zap.SugaredLogger) (*DefaultProvider, error) {
	if wsFactory == nil {
		return nil, fmt.Errorf("workspace factory cannot be nil")
	}
	return &DefaultProvider{
		wsFactory: wsFactory,
		logger:    logger,
	}, nil
}

func (p *DefaultProvider) WithFilter(f Filter) Provider {
	p.filters = append(p.filters, f)
	return p
}

func (p *DefaultProvider) RenderCRD(version string) ([]*Manifest, error) {
	ws, err := p.wsFactory.Get(version)
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

			crdData, err := fileUtils.ReadFile(path)
			if err != nil {
				return err
			}

			unstructs, err := reconcilerK8s.ToUnstructured(crdData, true)
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
	wsDir, err := p.workspaceDir(component)
	if err != nil {
		return nil, err
	}

	helmClient, err := NewHelmClient(wsDir, p.logger)
	if err != nil {
		return nil, err
	}

	manifest, err := helmClient.Render(component)
	if err != nil {
		return nil, err
	}

	for _, f := range p.filters {
		manifest, err = f(manifest)
		if err != nil {
			return nil, err
		}
	}

	return &Manifest{
		Type:     HelmChart,
		Name:     component.name,
		Manifest: manifest,
	}, nil
}

func (p *DefaultProvider) Configuration(component *Component) (map[string]interface{}, error) {
	wsDir, err := p.workspaceDir(component)
	if err != nil {
		return nil, err
	}

	helmClient, err := NewHelmClient(wsDir, p.logger)
	if err != nil {
		return nil, err
	}

	return helmClient.Configuration(component)
}

func (p *DefaultProvider) workspaceDir(component *Component) (string, error) {
	if component.url == "" {
		//is a Kyma component
		ws, err := p.wsFactory.Get(component.version)
		if err != nil {
			return "", err
		}
		return ws.ResourceDir, nil
	}

	//is an external component
	ws, err := p.wsFactory.GetExternalComponent(component)
	if err != nil {
		return "", err
	}
	return ws.WorkspaceDir, nil
}
