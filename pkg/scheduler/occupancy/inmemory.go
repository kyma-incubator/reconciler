package occupancy

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"sync"
	"time"
)

type InMemoryOccupancyRepository struct {
	occupancies map[string]*model.WorkerPoolOccupancyEntity
	sync.Mutex
}

func NewInMemoryOccupancyRepository() Repository {
	return &InMemoryOccupancyRepository{
		occupancies: make(map[string]*model.WorkerPoolOccupancyEntity),
	}
}

func (r *InMemoryOccupancyRepository) WithTx(tx *db.TxConnection) (Repository, error) {
	return r, nil
}

func (r *InMemoryOccupancyRepository) CreateWorkerPoolOccupancy(poolID, component string, runningWorkers, poolSize int) (*model.WorkerPoolOccupancyEntity, error) {
	r.Lock()
	defer r.Unlock()

	occupancyEntity := &model.WorkerPoolOccupancyEntity{
		WorkerPoolID:       poolID,
		Component:          component,
		RunningWorkers:     int64(runningWorkers),
		WorkerPoolCapacity: int64(poolSize),
		Created:            time.Now().UTC(),
	}
	r.occupancies[poolID] = occupancyEntity
	return occupancyEntity, nil
}

func (r *InMemoryOccupancyRepository) FindWorkerPoolOccupancyByID(poolID string) (*model.WorkerPoolOccupancyEntity, error) {
	r.Lock()
	defer r.Unlock()

	occupancyEntity, ok := r.occupancies[poolID]
	if !ok {
		return nil, fmt.Errorf("could not find a worker pool occupancy with a poolID: %s", poolID)
	}
	return occupancyEntity, nil
}

func (r *InMemoryOccupancyRepository) UpdateWorkerPoolOccupancy(poolID string, runningWorkers int) error {

	occupancyEntity, err := r.FindWorkerPoolOccupancyByID(poolID)
	if err != nil {
		return err
	}

	r.Lock()
	defer r.Unlock()
	//copy entity to avoid race conditions
	occCopy := *occupancyEntity
	if int64(runningWorkers) > occCopy.WorkerPoolCapacity {
		return fmt.Errorf("invalid number of running workers, should be less that worker pool capacity: "+
			"(running: %d, capacity:%d)", runningWorkers, occCopy.WorkerPoolCapacity)
	}
	occCopy.RunningWorkers = int64(runningWorkers)
	r.occupancies[poolID] = &occCopy
	return nil
}

func (r *InMemoryOccupancyRepository) CreateOrUpdateWorkerPoolOccupancy(poolID, component string, runningWorkers, poolSize int) (bool, error) {
	occupancyEntity, err := r.FindWorkerPoolOccupancyByID(poolID)
	if err != nil {
		_, err := r.CreateWorkerPoolOccupancy(poolID, component, runningWorkers, poolSize)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	if component != occupancyEntity.Component || int64(poolSize) != occupancyEntity.WorkerPoolCapacity {
		return false, fmt.Errorf("component '%s' with poolID '%s' and poolSize '%d' not found", component, poolID, poolSize)
	}
	err = r.UpdateWorkerPoolOccupancy(poolID, runningWorkers)
	if err != nil {
		return false, err
	}
	return false, nil
}

func (r *InMemoryOccupancyRepository) GetWorkerPoolOccupancies() ([]*model.WorkerPoolOccupancyEntity, error) {
	r.Lock()
	defer r.Unlock()

	if len(r.occupancies) == 0 {
		return nil, fmt.Errorf("unable to get worker pool occupancies: no record was found")
	}
	var occupancyEntities []*model.WorkerPoolOccupancyEntity
	for _, occupancyEntity := range r.occupancies {
		occupancyEntities = append(occupancyEntities, occupancyEntity)
	}

	return occupancyEntities, nil
}

func (r *InMemoryOccupancyRepository) RemoveWorkerPoolOccupancy(poolID string) error {

	_, err := r.FindWorkerPoolOccupancyByID(poolID)
	if err != nil {
		return err
	}

	r.Lock()
	defer r.Unlock()

	delete(r.occupancies, poolID)
	return nil
}

func (r *InMemoryOccupancyRepository) GetComponentList() ([]string, error) {

	err := r.emptyMapCheck()
	if err != nil {
		return nil, err
	}

	r.Lock()
	defer r.Unlock()

	var componentList []string
	for _, occupancyEntity := range r.occupancies {
		componentList = append(componentList, occupancyEntity.Component)
	}
	return componentList, nil
}

func (r *InMemoryOccupancyRepository) GetMeanWorkerPoolOccupancyByComponent(component string) (float64, error) {

	err := r.emptyMapCheck()
	if err != nil {
		return 0, err
	}

	r.Lock()
	defer r.Unlock()

	var aggregatedCapacity int64
	var aggregatedUsage int64
	for _, occupancyEntity := range r.occupancies {
		if occupancyEntity.Component == component {
			aggregatedUsage += occupancyEntity.RunningWorkers
			aggregatedCapacity += occupancyEntity.WorkerPoolCapacity
		}
	}
	aggregatedOccupancy := 100 * float64(aggregatedUsage) / float64(aggregatedCapacity)
	return aggregatedOccupancy, nil
}

func (r *InMemoryOccupancyRepository) emptyMapCheck() error {
	r.Lock()
	defer r.Unlock()

	if len(r.occupancies) == 0 {
		return fmt.Errorf("unable to calculate worker pool capacity: no record was found")
	}
	return nil
}
