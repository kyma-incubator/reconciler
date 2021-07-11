package chart

import (
	"fmt"
	"strings"

	"github.com/kyma-incubator/hydroform/parallel-install/pkg/components"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/config"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/deployment"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/overrides"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
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

func (p *Provider) loggerAdapter() (*HydroformLoggerAdapter, error) {
	logger, err := log.NewLogger(p.debug)
	if err != nil {
		return nil, err
	}
	return NewHydroformLoggerAdapter(logger), nil
}

func (p *Provider) Manifests(state *cluster.State, opts *Options) ([]*components.Manifest, error) {
	//TODO: add caching check here
	p.logger.Debug(fmt.Sprintf("Getting workspace for Kyma '%s'", state.Configuration.KymaVersion))
	ws, err := p.wsFactory.Get(state.Configuration.KymaVersion)
	if err != nil {
		return nil, err
	}

	return p.renderManifests(state, ws, opts)
}

func (p *Provider) renderManifests(state *cluster.State, ws *workspace.Workspace, opts *Options) ([]*components.Manifest, error) {
	if err := opts.validate(); err != nil {
		return nil, errors.Wrap(err, "Invalid provider options defined")
	}

	//get logger
	logger, err := p.loggerAdapter()
	if err != nil {
		return nil, err
	}

	//get component list
	kebComps, err := state.Configuration.GetComponents()
	if err != nil {
		return nil, err
	}
	compList, err := p.componentList(ws, kebComps)
	if err != nil {
		return nil, err
	}

	//get overrides
	builder, err := p.overrides(kebComps)
	if err != nil {
		return nil, err
	}

	//get kubeconfig
	kubeCfg, err := state.Kubeconfig.Get()
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
		Profile:                       state.Configuration.KymaProfile,
		Verbose:                       p.debug,
		KubeconfigSource: config.KubeconfigSource{
			Path:    "",
			Content: kubeCfg,
		},
		Version: state.Configuration.KymaVersion,
	}
	templating, err := deployment.NewTemplating(cfg, builder)
	if err != nil {
		return nil, err
	}

	return templating.Render()
}

//componentList will read the component list from the workspace and align the namespaces with the values retrieved from KEB
func (p *Provider) componentList(ws *workspace.Workspace, kebComps []*keb.Components) (*config.ComponentList, error) {
	compList, err := config.NewComponentList(ws.ComponentFile)
	if err != nil {
		return nil, err
	}
	kebCompsMap := p.kebComponentMap(kebComps)
	for idx, comp := range compList.Prerequisites {
		if kebComp, ok := kebCompsMap[comp.Name]; ok {
			if kebComp.Namespace != "" {
				p.logger.Debug(
					fmt.Sprintf("Updating namespace of prerequisite-component '%s' with namespace provided by KEB: '%s' => '%s'",
						comp.Name, comp.Namespace, kebComp.Namespace))
				compList.Prerequisites[idx].Namespace = kebComp.Namespace
			}
		}
	}
	for idx, comp := range compList.Components {
		if kebComp, ok := kebCompsMap[comp.Name]; ok {
			if kebComp.Namespace != "" {
				p.logger.Debug(
					fmt.Sprintf("Updating namespace of component '%s' with namespace provided by KEB: '%s' => '%s'",
						comp.Name, comp.Namespace, kebComp.Namespace))
				compList.Components[idx].Namespace = kebComp.Namespace
			}
		}
	}
	return compList, nil
}

func (p *Provider) kebComponentMap(kebComps []*keb.Components) map[string]*keb.Components {
	result := make(map[string]*keb.Components, len(kebComps))
	for _, kebComp := range kebComps {
		result[kebComp.Component] = kebComp
	}
	return result
}

func (p *Provider) overrides(kebComps []*keb.Components) (*overrides.Builder, error) {
	overrides := &overrides.Builder{}
	for _, kebComp := range kebComps {
		for _, kebCompConf := range kebComp.Configuration {
			if err := overrides.AddOverrides(kebComp.Component, p.kebConfToMap(kebCompConf)); err != nil {
				return nil, err
			}
		}
	}
	return overrides, nil
}

func (p *Provider) kebConfToMap(kebConf keb.Configuration) map[string]interface{} {
	result := make(map[string]interface{})
	tokens := strings.Split(kebConf.Key, ".")
	lastNestedMap := result
	for depth, token := range tokens {
		switch depth {
		case len(tokens) - 1: //last token reached, stop nesting
			lastNestedMap[token] = kebConf.Value
		default:
			lastNestedMap[token] = make(map[string]interface{})
			lastNestedMap = lastNestedMap[token].(map[string]interface{})
		}
	}
	return result
}
