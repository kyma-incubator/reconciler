package cluster

import (
	"time"

	"github.com/kyma-incubator/reconciler/pkg/keb"
)

type MockInventory struct {
	ClustersToReconcileResult []*State
	ClustersNotReadyResult    []*State
	GetResult                 *State
	GetLatestResult           *State
	CreateOrUpdateResult      *State
	DeleteResult              error
	UpdateStatusResult        error
	ChangesResult             []*State
}

func (i *MockInventory) CreateOrUpdate(cluster *keb.Cluster) (*State, error) {
	return i.CreateOrUpdateResult, nil
}

func (i *MockInventory) UpdateStatus(State *State) error {
	return i.UpdateStatusResult
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

func (i *MockInventory) ClustersToReconcile() ([]*State, error) {
	return i.ClustersToReconcileResult, nil
}

func (i *MockInventory) ClustersNotReady() ([]*State, error) {
	return i.ClustersNotReadyResult, nil
}

func (i *MockInventory) Changes(cluster string, offset time.Duration) ([]*State, error) {
	return i.ChangesResult, nil
}
