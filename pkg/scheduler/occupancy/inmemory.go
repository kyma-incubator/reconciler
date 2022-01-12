package occupancy

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"sync"
	"time"
)

type InMemoryOccupancyRepository struct {
	occupancies map[string]*model.WorkerPoolOccupancyEntity
	mu          sync.Mutex
}

func NewInMemoryWorkerRepository() Repository {
	return &InMemoryOccupancyRepository{
		occupancies: make(map[string]*model.WorkerPoolOccupancyEntity),
	}
}

func (r *InMemoryOccupancyRepository) CreateWorkerPoolOccupancy(component string, poolSize int) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	poolId := uuid.NewString()
	//create worker-pool occupancy
	occupancyEntity := &model.WorkerPoolOccupancyEntity{
		WorkerPoolID:       poolId,
		Component: component,
		RunningWorkers:     0,
		WorkerPoolCapacity: int64(poolSize),
		Created:            time.Now().UTC(),
	}
	r.occupancies[poolId] = occupancyEntity
	return poolId, nil
}

func (r *InMemoryOccupancyRepository) UpdateWorkerPoolOccupancy(poolId string, runningWorkers int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	//get worker-pool occupancy by WorkerPoolID
	occupancyEntity, ok := r.occupancies[poolId]
	if !ok {
		return fmt.Errorf("could not find a worker pool occupancy with a poolID: %s", poolId)
	}

	//copy entity to avoid race conditions
	occCopy := *occupancyEntity
	if int64(runningWorkers) > occCopy.WorkerPoolCapacity {
		return fmt.Errorf("invalid number of running workers, should be less that worker pool capacity: "+
			"(running: %d, capacity:%d)", runningWorkers, occCopy.WorkerPoolCapacity)
	}
	occCopy.RunningWorkers = int64(runningWorkers)
	r.occupancies[poolId] = &occCopy
	return nil
}

func (r *InMemoryOccupancyRepository) GetMeanWorkerPoolOccupancy() (float64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.occupancies) == 0 {
		return 0, fmt.Errorf("unable to calculate worker pool ocuppancy: no worker pool was found")
	}

	var aggregatedCapacity int64
	var aggregatedUsage int64
	for _, occupancyEntity := range r.occupancies {
		aggregatedUsage += occupancyEntity.RunningWorkers
		aggregatedCapacity += occupancyEntity.WorkerPoolCapacity
	}
	aggregatedOccupancy := 100 * float64(aggregatedUsage) / float64(aggregatedCapacity)
	return aggregatedOccupancy, nil
}

func (r *InMemoryOccupancyRepository) RemoveWorkerPoolOccupancy(poolId string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	//get worker-pool occupancy by WorkerPoolID
	_, ok := r.occupancies[poolId]
	if !ok {
		return fmt.Errorf("could not find a worker pool occupancy with a poolID: %s", poolId)
	}

	delete(r.occupancies, poolId)
	return nil
}

func (r *InMemoryOccupancyRepository) GetComponentList() ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.occupancies) == 0 {
		return nil, fmt.Errorf("unable to get component list: no component was found")
	}

	var componentList []string
	for _, occupancyEntity := range r.occupancies {
		componentList = append(componentList, occupancyEntity.Component)
	}
	return componentList, nil
}

func (r *InMemoryOccupancyRepository) GetMeanWorkerPoolOccupancyByComponent(component string) (float64, error){
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.occupancies) == 0 {
		return 0, fmt.Errorf("unable to calculate worker pool capacity: database is empty")
	}

	var aggregatedCapacity int64
	var aggregatedUsage int64
	for _, occupancyEntity := range r.occupancies {
		if occupancyEntity.Component == component{
			aggregatedUsage += occupancyEntity.RunningWorkers
			aggregatedCapacity += occupancyEntity.WorkerPoolCapacity
		}
	}
	aggregatedOccupancy := 100 * float64(aggregatedUsage) / float64(aggregatedCapacity)
	return aggregatedOccupancy, nil
}

func (r *InMemoryOccupancyRepository) WithTx(tx *db.TxConnection) (Repository, error) {
	return r, nil
}
