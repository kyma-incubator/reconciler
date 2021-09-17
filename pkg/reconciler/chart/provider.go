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

type Provider struct {
	wsFactory *workspace.Factory
	logger    *zap.SugaredLogger
}

func NewProvider(wsFactory *workspace.Factory, logger *zap.SugaredLogger) (*Provider, error) {
	if wsFactory == nil {
		return nil, fmt.Errorf("workspace factory cannot be nil")
	}
	return &Provider{
		wsFactory: wsFactory,
		logger:    logger,
	}, nil
}

func (p *Provider) RenderCRD(version string) ([]*Manifest, error) {
	ws, err := p.newWorkspace(version)
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

func (p *Provider) RenderManifest(component *Component) (*Manifest, error) {
	ws, err := p.newWorkspace(component.version)
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

func (p *Provider) Configuration(component *Component) (map[string]interface{}, error) {
	ws, err := p.newWorkspace(component.version)
	if err != nil {
		return nil, err
	}

	helmClient, err := NewHelmClient(ws.ResourceDir, p.logger)
	if err != nil {
		return nil, err
	}

	return helmClient.Configuration(component)
}

func (p *Provider) newWorkspace(version string) (*workspace.Workspace, error) {
	p.logger.Debugf("Getting workspace for Kyma '%s'", version)
	ws, err := p.wsFactory.Get(version)
	if err != nil {
		p.logger.Warnf("Failed to retrieve workspace for Kyma '%s': %s", version, err)
	}
	return ws, err
}
