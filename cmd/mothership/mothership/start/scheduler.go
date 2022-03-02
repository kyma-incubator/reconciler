package cmd

import (
	"context"
	"strconv"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/service"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/worker"
	"github.com/spf13/viper"
)

func startScheduler(ctx context.Context, o *Options) error {

	runtimeBuilder := service.NewRuntimeBuilder(o.Registry.ReconciliationRepository(), logger.NewLogger(o.Verbose))
	ds, err := service.NewDeleteStrategy(o.Config.Scheduler.DeleteStrategy)
	if err != nil {
		return err
	}

	return runtimeBuilder.
		RunRemote(o.Registry.Connection(), o.Registry.Inventory(), o.Registry.OccupancyRepository(), o.Config).
		WithWorkerPoolConfig(&worker.Config{
			MaxParallelOperations: o.MaxParallelOperations,
			PoolSize:              o.Workers,
			//check-interval should be greater than "max-retires * retry-delay" to avoid queuing
			//of workers in case that component-reconciler isn't reachable
			OperationCheckInterval: 30 * time.Second,
			InvokerMaxRetries:      2,
			InvokerRetryDelay:      10 * time.Second,
		}).
		WithSchedulerConfig(
			&service.SchedulerConfig{
				InventoryWatchInterval:   o.WatchInterval,
				ClusterReconcileInterval: o.ClusterReconcileInterval,
				ClusterQueueSize:         10,
				DeleteStrategy:           ds,
				PreComponents:            o.Config.Scheduler.PreComponents,
			}).
		WithBookkeeperConfig(&service.BookkeeperConfig{
			OperationsWatchInterval: 45 * time.Second,
			OrphanOperationTimeout:  o.OrphanOperationTimeout,
		}).
		WithCleanerConfig(&service.CleanerConfig{
			PurgeEntitiesOlderThan:       o.PurgeEntitiesOlderThan,
			CleanerInterval:              o.CleanerInterval,
			KeepLatestEntitiesCount:      uintOrDie(o.KeepLatestEntitiesCount),
			KeepUnsuccessfulEntitiesDays: uintOrDie(o.KeepUnsuccessfulEntitiesDays),
		}).
		Run(ctx)
}

func parseSchedulerConfig(configFile string) (*config.Config, error) {
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return &config.Config{}, err
	}

	var cfg config.Config
	return &cfg, viper.UnmarshalKey("mothership", &cfg)
}

func uintOrDie(v int) uint {
	if v < 0 {
		panic("Can't convert negative value: '" + strconv.Itoa(v) + "' to the uint type")
	}
	return uint(v)
}
