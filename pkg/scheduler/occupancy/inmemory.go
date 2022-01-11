package occupancy

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"sync"
	"time"
)

type InMemoryWorkerRepository struct {
	occupancies     map[string]*model.WorkerPoolOccupancyEntity
	mu              sync.Mutex
}

func NewInMemoryWorkerRepository() Repository {
	return &InMemoryWorkerRepository{
		occupancies:     make(map[string]*model.WorkerPoolOccupancyEntity),
	}
}

func (r *InMemoryWorkerRepository) CreateWorkerPoolOccupancy(poolSize int) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	poolId := uuid.NewString()
	//create worker-pool occupancy
	occupancyEntity := &model.WorkerPoolOccupancyEntity{
		WorkerPoolID:       poolId,
		RunningWorkers:     0,
		WorkerPoolCapacity: int64(poolSize),
		Created:            time.Now().UTC(),
	}
	r.occupancies[poolId] = occupancyEntity
	return poolId, nil
}

func (r *InMemoryWorkerRepository) UpdateWorkerPoolOccupancy(poolId string, runningWorkers int) error {
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
func (r *InMemoryWorkerRepository) GetMeanWorkerPoolOccupancy() (float64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.occupancies) == 0 {
		return 0, fmt.Errorf("unable to calculate worker pool capacity: database is empty")
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

func (r *InMemoryWorkerRepository) RemoveWorkerPoolOccupancy(poolId string) error {
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

func (r *InMemoryWorkerRepository) WithTx(tx *db.TxConnection) (Repository, error) {
	return r, nil
}