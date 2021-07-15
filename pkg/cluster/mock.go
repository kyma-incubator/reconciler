package cluster

import (
	"time"

	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

type MockInventory struct {
	ClustersToReconcileResult []*State
	ClustersNotReadyResult    []*State
	GetResult                 *State
	GetLatestResult           *State
	CreateOrUpdateResult      *State
	DeleteResult              error
	UpdateStatusResult        *State
	ChangesResult             []*StatusChange
}

func (i *MockInventory) CreateOrUpdate(contractVersion int64, cluster *keb.Cluster) (*State, error) {
	return i.CreateOrUpdateResult, nil
}

func (i *MockInventory) UpdateStatus(State *State, status model.Status) (*State, error) {
	return i.UpdateStatusResult, nil
}

func (i *MockInventory) Delete(cluster string) error {
	return i.DeleteResult
}

func (i *MockInventory) Get(cluster string, configVersion int64) (*State, error) {
	return i.GetResult, nil
}

func (i *MockInventory) GetLatest(cluster string) (*State, error) {
	return i.GetLatestResult, nil
}

func (i *MockInventory) ClustersToReconcile(reconcileInterval time.Duration) ([]*State, error) {
	return i.ClustersToReconcileResult, nil
}

func (i *MockInventory) ClustersNotReady() ([]*State, error) {
	return i.ClustersNotReadyResult, nil
}

func (i *MockInventory) StatusChanges(cluster string, offset time.Duration) ([]*StatusChange, error) {
	return i.ChangesResult, nil
}
