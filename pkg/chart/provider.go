package chart

import (
	"time"

	"github.com/kyma-incubator/hydroform/parallel-install/pkg/components"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/config"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/deployment"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/overrides"
	"github.com/kyma-incubator/reconciler/pkg/db"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/workspace"
)

type Provider struct {
	Debug bool
}

func (p *Provider) loggerAdapter() (*HydroformLoggerAdapter, error) {
	logger, err := log.NewLogger(p.Debug)
	if err != nil {
		return nil, err
	}
	return NewHydroformLoggerAdapter(logger), nil
}

func (p *Provider) Manifests(cluster *db.DatabaseEntity, ws *workspace.Workspace) ([]*components.Manifest, error) {
	logger, err := p.loggerAdapter()
	if err != nil {
		return nil, err
	}

	compList, err := config.NewComponentList(ws.ComponentFile)
	if err != nil {
		return nil, err
	}

	cfg := &config.Config{
		WorkersCount:                  4,
		CancelTimeout:                 20 * time.Minute,
		QuitTimeout:                   25 * time.Minute,
		HelmTimeoutSeconds:            60 * 8,
		BackoffInitialIntervalSeconds: 3,
		BackoffMaxElapsedTimeSeconds:  60 * 5,
		Log:                           logger,
		HelmMaxRevisionHistory:        10,
		ComponentList:                 compList,
		ResourcePath:                  ws.ResourceDir,
		InstallationResourcePath:      ws.InstallationResourceDir,
		KubeconfigSource: config.KubeconfigSource{
			Path:    cluster.KubeConfig, //FIXME
			Content: "",
		},
		Version: cluster.Version, //FIXME
	}

	builder := &overrides.Builder{} //TODO: merge config from cluster model

	templating, err := deployment.NewTemplating(cfg, builder)
	if err != nil {
		return nil, err
	}

	return templating.Render()
}
