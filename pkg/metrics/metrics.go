package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/features"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func RegisterProcessingDuration(reconciliations reconciliation.Repository, logger *zap.SugaredLogger) error {
	if features.Enabled(features.ProcessingDurationMetric) {
		processingDurationCollector := NewProcessingDurationCollector(reconciliations, logger)
		err := prometheus.Register(processingDurationCollector)
		switch err := err.(type) {
		case prometheus.AlreadyRegisteredError:
			logger.Warnf("skipping registration of processing duration metrics as they were already registered, existing: %v",
				err.ExistingCollector)
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func RegisterWaitingAndNotReadyReconciliations(inventory cluster.Inventory, logger *zap.SugaredLogger) error {
	reconciliationWaitingCollector := NewReconciliationWaitingCollector(inventory, logger)
	reconciliationNotReadyCollector := NewReconciliationNotReadyCollector(inventory, logger)
	err := prometheus.Register(reconciliationWaitingCollector)
	if err == nil {
		err = prometheus.Register(reconciliationNotReadyCollector)
	}
	switch err := err.(type) {
	case prometheus.AlreadyRegisteredError:
		logger.Warnf("skipping registration of waiting/ready metrics as they were already registered, existing: %v",
			err.ExistingCollector)
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

func RegisterDbPool(connPool db.Connection, logger *zap.SugaredLogger) error {
	dbPoolMetricsCollector := NewDbPoolCollector(connPool, logger)
	err := prometheus.Register(dbPoolMetricsCollector)
	switch err := err.(type) {
	case prometheus.AlreadyRegisteredError:
		logger.Warnf("skipping registration of occupancy metrics as they were already registered, existing: %v",
			err.ExistingCollector)
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

func RegisterOccupancy(occupancyRepo occupancy.Repository, reconcilers map[string]config.ComponentReconciler, logger *zap.SugaredLogger) error {
	if features.Enabled(features.WorkerpoolOccupancyTracking) {
		err := prometheus.Register(NewWorkerPoolOccupancyCollector(occupancyRepo, reconcilers, logger))
		switch err := err.(type) {
		case prometheus.AlreadyRegisteredError:
			logger.Warnf("skipping registration of occupancy metrics as they were already registered, existing: %v",
				err.ExistingCollector)
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}
