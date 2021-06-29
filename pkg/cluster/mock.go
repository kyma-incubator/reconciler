package cluster

type MockInventory struct {
	ClustersToReconcileResult []*State
	GetResult                 *State
	CreateOrUpdateResult      *State
	DeleteResult              error
	UpdateStatusResult        error
}

func (i *MockInventory) CreateOrUpdate(cluster *Cluster) (*State, error) {
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

func (i *MockInventory) ClustersToReconcile() ([]*State, error) {
	return i.ClustersToReconcileResult, nil
}
