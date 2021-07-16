package chart

import (
	"fmt"
	"os"
	"strings"

	"github.com/kyma-incubator/hydroform/parallel-install/pkg/components"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/config"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/deployment"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/overrides"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/workspace"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Provider struct {
	debug     bool
	wsFactory *workspace.Factory
	logger    *zap.Logger
}

func NewProvider(wsFactory *workspace.Factory, debug bool) (*Provider, error) {
	if wsFactory == nil {
		return nil, fmt.Errorf("Workspace factory cannot be nil")
	}
	logger, err := log.NewLogger(debug)
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
	logger, err := log.NewLogger(p.debug)
	if err != nil {
		return nil, err
	}
	return NewHydroformLoggerAdapter(logger), nil
}

func (p *Provider) Manifests(compSet *ComponentSet, opts *Options) ([]*components.Manifest, error) {
	//TODO: add caching check here
	p.logger.Debug(fmt.Sprintf("Getting workspace for Kyma '%s'", compSet.version))
	ws, err := p.wsFactory.Get(compSet.version)
	if err != nil {
		return nil, err
	}

	return p.renderManifests(compSet, ws, opts)
}

func (p *Provider) renderManifests(compSet *ComponentSet, ws *workspace.Workspace, opts *Options) ([]*components.Manifest, error) {
	if err := opts.validate(); err != nil {
		return nil, errors.Wrap(err, "Invalid provider options defined")
	}

	//get logger
	logger, err := p.loggerAdapter()
	if err != nil {
		return nil, err
	}

	//get component list
	compList, err := p.componentList(ws, compSet.components)
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
		ComponentList:                 compList,
		ResourcePath:                  ws.ResourceDir,
		InstallationResourcePath:      ws.InstallationResourceDir,
		Profile:                       compSet.profile,
		Verbose:                       p.debug,
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

	return templating.Render()
}

//componentList will read the component list from the workspace and align the namespaces with the values retrieved from KEB
func (p *Provider) componentList(ws *workspace.Workspace, comps []*Component) (*config.ComponentList, error) {
	compList, err := config.NewComponentList(ws.ComponentFile)
	if err != nil {
		return nil, err
	}
	compsMap := p.componentMap(comps)
	for idx, clComp := range compList.Prerequisites {
		if comp, ok := compsMap[clComp.Name]; ok {
			if comp.namespace != "" {
				p.logger.Debug(
					fmt.Sprintf("Updating namespace of prerequisite-component '%s' with namespace provided by KEB: '%s' => '%s'",
						clComp.Name, clComp.Namespace, comp.namespace))
				compList.Prerequisites[idx].Namespace = comp.namespace
			}
		}
	}
	for idx, clComp := range compList.Components {
		if comp, ok := compsMap[clComp.Name]; ok {
			if comp.namespace != "" {
				p.logger.Debug(
					fmt.Sprintf("Updating namespace of component '%s' with namespace provided by KEB: '%s' => '%s'",
						clComp.Name, clComp.Namespace, comp.namespace))
				compList.Components[idx].Namespace = comp.namespace
			}
		}
	}
	return compList, nil
}

func (p *Provider) componentMap(comps []*Component) map[string]*Component {
	result := make(map[string]*Component, len(comps))
	for _, comp := range comps {
		result[comp.name] = comp
	}
	return result
}

func (p *Provider) overrides(comps []*Component) (*overrides.Builder, error) {
	overrides := &overrides.Builder{}
	for _, comp := range comps {
		for key, value := range comp.configuration {
			if err := overrides.AddOverrides(comp.name, p.nestedConfMap(key, value)); err != nil {
				return nil, err
			}
		}
	}
	return overrides, nil
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
