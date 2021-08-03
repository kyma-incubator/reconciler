package chart

import (
	"fmt"
	"os"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"

	"github.com/kyma-incubator/hydroform/parallel-install/pkg/components"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/config"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/deployment"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/overrides"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	log "github.com/kyma-incubator/reconciler/pkg/reconciler/logger"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Provider struct {
	debug     bool
	wsFactory *workspace.Factory
	logger    *zap.SugaredLogger
}

func NewProvider(wsFactory *workspace.Factory, correlationID string, debug bool) (*Provider, error) {
	if wsFactory == nil {
		return nil, fmt.Errorf("Workspace factory cannot be nil")
	}
	logger, err := log.NewLogger(correlationID, debug)
	if err != nil {
		return nil, err
	}
	return &Provider{
		debug:     debug,
		wsFactory: wsFactory,
		logger:    logger,
	}, nil
}

func (p *Provider) ChangeWorkspace(wsDir string) error {
	if !file.DirExists(wsDir) {
		if err := os.MkdirAll(wsDir, 0755); err != nil {
			return err
		}
	}
	p.wsFactory.StorageDir = wsDir
	return nil
}

func (p *Provider) loggerAdapter() (*HydroformLoggerAdapter, error) {
	logger, err := log.NewLogger("", p.debug)
	if err != nil {
		return nil, err
	}
	return NewHydroformLoggerAdapter(logger), nil
}

func (p *Provider) Manifests(compSet *ComponentSet, includeCRD bool, opts *Options) ([]*components.Manifest, error) {
	//TODO: add caching check here
	p.logger.Debugf("Getting workspace for Kyma '%s'", compSet.version)
	ws, err := p.wsFactory.Get(compSet.version)
	if err != nil {
		p.logger.Warnf("Failed to retrieve workspace for Kyma '%s': %s", compSet.version, err)
		return nil, err
	}

	var result []*components.Manifest
	if len(compSet.components) > 0 {
		manifests, err := p.renderManifests(compSet, ws, opts)
		if err != nil {
			return nil, err
		}
		result = append(result, manifests...)
	}
	if includeCRD {
		crds, err := p.renderCrds(compSet, ws, opts)
		if err != nil {
			return nil, err
		}
		result = append(result, crds...)
	}

	return result, nil
}

func (p *Provider) renderManifests(compSet *ComponentSet, ws *workspace.Workspace, opts *Options) ([]*components.Manifest, error) {
	return p.render(compSet, false, ws, opts)
}

func (p *Provider) renderCrds(compSet *ComponentSet, ws *workspace.Workspace, opts *Options) ([]*components.Manifest, error) {
	return p.render(compSet, true, ws, opts)
}

func (p *Provider) render(compSet *ComponentSet, renderCrds bool, ws *workspace.Workspace, opts *Options) ([]*components.Manifest, error) {
	if err := opts.validate(); err != nil {
		return nil, errors.Wrap(err, "Invalid provider options defined")
	}

	//get logger
	logger, err := p.loggerAdapter()
	if err != nil {
		return nil, err
	}

	//get overrides
	builder, err := p.overrides(compSet.components)
	if err != nil {
		return nil, err
	}

	//get templating instance
	cfg := &config.Config{
		WorkersCount:                  opts.WorkersCount,
		CancelTimeout:                 opts.CancelTimeout,
		QuitTimeout:                   opts.QuitTimeout,
		HelmTimeoutSeconds:            60,
		BackoffInitialIntervalSeconds: 3,
		BackoffMaxElapsedTimeSeconds:  45,
		Log:                           logger,
		HelmMaxRevisionHistory:        10,
		ComponentList:                 p.componentList(compSet.components),
		ResourcePath:                  ws.ResourceDir,
		InstallationResourcePath:      ws.InstallationResourceDir,
		Profile:                       compSet.profile,
		KubeconfigSource: config.KubeconfigSource{
			Path:    "",
			Content: compSet.kubeconfig,
		},
		Version: compSet.version,
	}
	templating, err := deployment.NewTemplating(cfg, builder)
	if err != nil {
		return nil, err
	}

	return templating.Render(renderCrds)
}

//componentList will create a new component list using the components provided by KEB
func (p *Provider) componentList(comps []*Component) *config.ComponentList {
	compList := &config.ComponentList{}
	for _, comp := range comps {
		p.logger.Debugf("Adding component '%s' with namespace '%s' to rendering scope",
			comp.name, comp.namespace)
		compList.Components = append(compList.Components, config.ComponentDefinition{
			Name:      comp.name,
			Namespace: comp.namespace,
		})
	}
	return compList
}

func (p *Provider) overrides(comps []*Component) (*overrides.Builder, error) {
	overrideBuilder := &overrides.Builder{}
	for _, comp := range comps {
		for key, value := range comp.configuration {
			if err := overrideBuilder.AddOverrides(comp.name, p.nestedConfMap(key, value)); err != nil {
				return nil, err
			}
		}
	}
	return overrideBuilder, nil
}

//nestedConfMap converts a key with dot-notation into a nested map (e.g. a.b.c=value become [a:[b:[c:value]]])
func (p *Provider) nestedConfMap(key string, value interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	tokens := strings.Split(key, ".")
	lastNestedMap := result
	for depth, token := range tokens {
		switch depth {
		case len(tokens) - 1: //last token reached, stop nesting
			lastNestedMap[token] = value
		default:
			lastNestedMap[token] = make(map[string]interface{})
			lastNestedMap = lastNestedMap[token].(map[string]interface{})
		}
	}
	return result
}
