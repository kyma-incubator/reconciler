package scheduler

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"go.uber.org/zap"
)

const (
	channelSize            = 100
	defaultProgressTimeout = 1 * time.Hour
)

type ClusterStatusUpdater struct {
	inventory     cluster.Inventory
	clusterState  cluster.State
	updateChannel chan Update
	statusMap     map[string]string
	logger        *zap.SugaredLogger
}

type Update struct {
	component      string
	operationState string
}

func NewClusterStatusUpdater(inventory cluster.Inventory, clusterState cluster.State, components []*keb.Components, logger *zap.SugaredLogger) ClusterStatusUpdater {
	statusUpdater := ClusterStatusUpdater{inventory: inventory, clusterState: clusterState, logger: logger}
	statusUpdater.statusMap = make(map[string]string)
	for _, comp := range components {
		statusUpdater.statusMap[comp.Component] = StateInProgress
	}
	statusUpdater.reconciling()
	statusUpdater.updateChannel = make(chan Update, channelSize)
	return statusUpdater
}

func (su *ClusterStatusUpdater) Run() {
	timeout := time.After(defaultProgressTimeout)
	for {
		select {
		case update := <-su.updateChannel:
			su.statusMap[update.component] = update.operationState
			if update.operationState == StateDone {
				su.success()
			} else if update.operationState == StateError {
				su.error()
			}
			if su.isAllInEndState() {
				close(su.updateChannel)
				return
			}
		case <-timeout:
			su.logger.Warnf("cluster status updater reached timeout (%.0f secs)", defaultProgressTimeout.Seconds())
			return
		}
	}
}

func (su *ClusterStatusUpdater) Update(component string, operationState string) {
	su.updateChannel <- Update{component, operationState}
}

func (su *ClusterStatusUpdater) reconciling() {
	if err := su.statusChangeAllowed(model.ClusterStatusReconciling); err != nil {
		su.logger.Warn(err)
		return
	}
	su.sendUpdate(model.ClusterStatusReconciling)
}

func (su *ClusterStatusUpdater) success() {
	if err := su.statusChangeAllowed(model.ClusterStatusReady); err != nil {
		su.logger.Warn(err)
		return
	}
	if su.isAllDone() {
		su.sendUpdate(model.ClusterStatusReady)
	}
}

func (su *ClusterStatusUpdater) error() {
	if err := su.statusChangeAllowed(model.ClusterStatusError); err != nil {
		su.logger.Warn(err)
		return
	}
	su.sendUpdate(model.ClusterStatusError)
}

func (su *ClusterStatusUpdater) sendUpdate(status model.Status) {
	_, err := su.inventory.UpdateStatus(&su.clusterState, status)
	if err != nil {
		su.logger.Infof("Failed to update cluster status as ready: %s", err)
	}
}

func (su *ClusterStatusUpdater) statusChangeAllowed(status model.Status) error {
	latestState, err := su.inventory.GetLatest(su.clusterState.Cluster.Cluster)
	if err != nil {
		return fmt.Errorf("failed to get the latest cluster status: %s", err)
	}

	if latestState.Status.Status == model.ClusterStatusError || latestState.Status.Status == model.ClusterStatusReady {
		return fmt.Errorf("cannot switch in '%s' status because we are already in final status '%s'", status, latestState.Status.Status)
	}
	return nil
}

func (su *ClusterStatusUpdater) isAllDone() bool {
	for _, state := range su.statusMap {
		if state != StateDone {
			return false
		}
	}
	return true
}

func (su *ClusterStatusUpdater) isAllInEndState() bool {
	for _, state := range su.statusMap {
		if state != StateDone && state != StateError {
			return false
		}
	}
	return true
}
