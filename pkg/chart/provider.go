package chart

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/hydroform/parallel-install/pkg/components"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/config"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/deployment"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/overrides"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/workspace"
	"github.com/pkg/errors"
)

const (
	defaultWorkersCount  = 4
	defaultCancelTimeout = 2 * time.Minute
	defaultQuitTimeout   = 3 * time.Minute
)

type Provider struct {
	Debug bool
}

type Options struct {
	WorkersCount  int
	CancelTimeout time.Duration
	QuitTimeout   time.Duration
}

func (o *Options) validate() error {
	if o.WorkersCount <= 0 {
		return fmt.Errorf("WorkersCount cannot be < 0")
	}
	if o.WorkersCount == 0 {
		o.WorkersCount = defaultWorkersCount
	}
	if o.CancelTimeout.Microseconds() == int64(0) {
		o.CancelTimeout = defaultCancelTimeout
	}
	if o.QuitTimeout.Microseconds() == int64(0) {
		o.QuitTimeout = defaultQuitTimeout
	}
	return nil
}

func (p *Provider) loggerAdapter() (*HydroformLoggerAdapter, error) {
	logger, err := log.NewLogger(p.Debug)
	if err != nil {
		return nil, err
	}
	return NewHydroformLoggerAdapter(logger), nil
}

func (p *Provider) Manifests(cluster *model.ClusterPropertyEntity /* << fix me */, ws *workspace.Workspace, opts *Options) ([]*components.Manifest, error) {
	if err := opts.validate(); err != nil {
		return nil, errors.Wrap(err, "Invalid options provided")
	}

	logger, err := p.loggerAdapter()
	if err != nil {
		return nil, err
	}

	compList, err := config.NewComponentList(ws.ComponentFile)
	if err != nil {
		return nil, err
	}

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

		//FIXME:
		// KubeconfigSource: config.KubeconfigSource{
		// 	Path:    cluster.KubeConfig, //FIXME
		// 	Content: "",
		// },
		// Version: cluster.Version, //FIXME
	}

	builder := &overrides.Builder{} //TODO: merge config from cluster model

	templating, err := deployment.NewTemplating(cfg, builder)
	if err != nil {
		return nil, err
	}

	return templating.Render()
}
