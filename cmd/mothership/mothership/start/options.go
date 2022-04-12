package cmd

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"

	"github.com/pkg/errors"

	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/ssl"
)

type Options struct {
	*cli.Options
	Port                           int
	SSLCrt                         string
	SSLKey                         string
	Workers                        int
	WatchInterval                  time.Duration
	OrphanOperationTimeout         time.Duration
	ClusterReconcileInterval       time.Duration
	PurgeEntitiesOlderThan         time.Duration
	CleanerInterval                time.Duration
	BookkeeperWatchInterval        time.Duration
	ReconciliationsKeepLatestCount int
	EntitiesMaxAgeDays             int
	StatusCleanupBatchSize         int
	CreateEncyptionKey             bool
	MaxParallelOperations          int
	AuditLog                       bool
	AuditLogFile                   string
	AuditLogTenantID               string
	StopAfterMigration             bool
	Config                         *config.Config
}

func NewOptions(o *cli.Options) *Options {
	return &Options{o,
		0,                //Port
		"",               //SSLCrt
		"",               //SSLKey
		0,                //Workers
		0 * time.Second,  //WatchInterval
		0 * time.Minute,  //Orphan timeout
		0 * time.Second,  //ClusterReconcileInterval
		0 * time.Minute,  //PurgeEntitiesOlderThan
		0 * time.Minute,  //CleanerInterval
		45 * time.Second, //BookkeeperWatchInterval
		0,                //ReconciliationsKeepLatestCount
		0,                //EntitiesMaxAgeDays
		0,                // StatusCleanupBatchSize
		false,            //CreateEncyptionKey
		0,                //MaxParallelOperations
		false,            //AuditLog
		"",               //AuditLogFile
		"",               //AuditLogTenant
		false,            //StopAfterMigration
		&config.Config{}, //Config
	}
}

func (o *Options) Validate() error {
	if o.Port <= 0 || o.Port > 65535 {
		return fmt.Errorf("port %d is out of range 1-65535", o.Port)
	}
	if o.Workers <= 0 {
		return errors.New("amount of workers cannot be <= 0")
	}
	if o.WatchInterval <= 0 {
		return errors.New("defined watch interval is <= 0")
	}
	if o.OrphanOperationTimeout <= 0 {
		return errors.New("defined orphan timeout cannot be <= 0")
	}
	if o.ClusterReconcileInterval <= 0 {
		return errors.New("cluster reconciliation interval cannot be <= 0")
	}
	if o.ReconciliationsKeepLatestCount < 0 {
		return errors.New("cleaner count of latest entities to keep cannot be < 0")
	}
	if o.EntitiesMaxAgeDays < 0 {
		return errors.New("cleaner count of days to keep unsuccessful entities cannot be < 0")
	}
	if o.StatusCleanupBatchSize < 1 {
		return errors.New("cluster status cleaner batch size cannot be < 1")
	}
	if o.MaxParallelOperations < 0 {
		return errors.New("maximal parallel reconciled components per cluster cannot be < 0")
	}
	if o.AuditLog {
		if o.AuditLogFile == "" {
			return errors.New("audit log file must be set if audit logging is enable")
		}
		if o.AuditLogTenantID == "" {
			return errors.New("audit log tenant-id must be set if audit logging is enable")

		}
	}
	return ssl.VerifyKeyPair(o.SSLCrt, o.SSLKey)
}
