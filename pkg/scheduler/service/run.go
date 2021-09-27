package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"go.uber.org/zap"
)

type RuntimeBuilder struct {
	reconRepo     reconciliation.Repository
	preComponents []string
	logger        *zap.SugaredLogger
}

func NewRuntimeBuilder(reconRepo reconciliation.Repository, preComponents []string) *RuntimeBuilder {
	return &RuntimeBuilder{
		reconRepo:     reconRepo,
		preComponents: preComponents,
	}
}

func (m *RuntimeBuilder) Debug() {
	m.logger = logger.NewLogger(true)
}

func (m *RuntimeBuilder) RunLocal() *runLocal {
	return &runLocal{m}
}

func (m *RuntimeBuilder) RunRemote(conn db.Connection, inventory cluster.Inventory) *runRemote {
	return &runRemote{m, conn, inventory, &Config{}}
}

func (m *RuntimeBuilder) newScheduler() *Scheduler {
	return NewScheduler(m.preComponents, m.logger)
}

type runLocal struct {
	*RuntimeBuilder
}

func (l *runLocal) Run(clusterState *cluster.State) error {
	return l.newScheduler().RunOnce(clusterState, l.reconRepo)
}

type runRemote struct {
	*RuntimeBuilder
	conn      db.Connection
	inventory cluster.Inventory
	config    *Config
}

func (r *runRemote) WithConfig(cfg *Config) *runRemote {
	r.config = cfg
	return r
}

func (r *runRemote) Run(ctx context.Context) error {
	transition := NewClusterStatusTransition(r.conn, r.inventory, r.reconRepo, r.logger)
	//TODO: start bookkeeper
	return r.newScheduler().Run(ctx, transition, r.config)
}
