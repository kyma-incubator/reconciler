package occupancy

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"time"
)

type PersistentWorkerRepository struct {
	*repository.Repository
}

func NewPersistentWorkerRepository(conn db.Connection, debug bool) (Repository, error) {
	repo, err := repository.NewRepository(conn, debug)
	if err != nil {
		return nil, err
	}
	return &PersistentWorkerRepository{repo}, nil
}

func (r *PersistentWorkerRepository) WithTx(tx *db.TxConnection) (Repository, error) {
	return NewPersistentWorkerRepository(tx, r.Debug)
}


func (r *PersistentWorkerRepository) CreateWorkerPoolOccupancy(poolSize int) (string, error) {

	dbOps := func(tx *db.TxConnection) (interface{}, error) {
		poolId := uuid.NewString()
		occupancyEntity := &model.WorkerPoolOccupancyEntity{
			WorkerPoolID:       poolId,
			RunningWorkers:     0,
			WorkerPoolCapacity: int64(poolSize),
			Created:            time.Now().UTC(),
		}

		createOccupancyQ, err := db.NewQuery(tx, occupancyEntity, r.Logger)
		if err != nil {
			return "", err
		}
		if err = createOccupancyQ.Insert().Exec(); err != nil {
			r.Logger.Errorf("ReconRepo failed to create new worker-pool occupancy entity: %s", err)
			return "", err
		}

		r.Logger.Debugf("ReconRepo created new worker-pool occupancy entity with poolID '%s'", occupancyEntity.WorkerPoolID)
		return occupancyEntity.WorkerPoolID, err
	}
	result, err := db.TransactionResult(r.Conn, dbOps, r.Logger)
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

func (r *PersistentWorkerRepository) UpdateWorkerPoolOccupancy(poolId string, runningWorkers int) error {

	dbOps := func(tx *db.TxConnection) error {

		findOccupancyQ, err := db.NewQuery(tx, &model.WorkerPoolOccupancyEntity{}, r.Logger)
		if err != nil {
			return err
		}
		whereCond := map[string]interface{}{"WorkerPoolID": poolId}
		databaseEntity, err := findOccupancyQ.Select().Where(whereCond).GetOne()
		if err != nil {
			if repository.IsNotFoundError(err) {
				r.Logger.Errorf("could not find a worker pool occupancy with a poolID: %s", poolId)
			}
			return err
		}

		occupancyEntity := databaseEntity.(*model.WorkerPoolOccupancyEntity)
		cvtdRunningWorkers := int64(runningWorkers)
		if cvtdRunningWorkers > occupancyEntity.WorkerPoolCapacity {
			return fmt.Errorf("invalid number of running workers, should be less that worker pool capacity: "+
				"(running: %d, capacity:%d)", runningWorkers, occupancyEntity.WorkerPoolCapacity)
		}
		occupancyEntity.RunningWorkers = int64(runningWorkers)
		updateOccupancyQ, err := db.NewQuery(tx, occupancyEntity, r.Logger)
		if err != nil {
			return err
		}
		if err = updateOccupancyQ.Update().Where(whereCond).Exec(); err != nil {
			r.Logger.Errorf("ReconRepo failed to update occupancy entity with poolID '%s': %s", occupancyEntity.WorkerPoolID, err)
			return err
		}

		r.Logger.Debugf("ReconRepo updated workersCnt of occupancy entity with poolID '%s' to '%d'", occupancyEntity.WorkerPoolID, runningWorkers)
		return err
	}
	return db.Transaction(r.Conn, dbOps, r.Logger)
}

func (r *PersistentWorkerRepository) GetMeanWorkerPoolOccupancy() (float64, error) {

	q, err := db.NewQuery(r.Conn, &model.WorkerPoolOccupancyEntity{}, r.Logger)
	if err != nil {
		return 0, err
	}

	occupancies, err := q.Select().GetMany()
	if err != nil {
		return 0, err
	}
	if len(occupancies) == 0 {
		return 0, fmt.Errorf("unable to calculate worker pool capacity: database is empty")
	}

	var aggregatedCapacity int64
	var aggregatedUsage int64
	for _, occupancy := range occupancies {
		occupancyEntity := occupancy.(*model.WorkerPoolOccupancyEntity)
		aggregatedUsage += occupancyEntity.RunningWorkers
		aggregatedCapacity += occupancyEntity.WorkerPoolCapacity
	}
	aggregatedOccupancy := 100 * float64(aggregatedUsage) / float64(aggregatedCapacity)
	return aggregatedOccupancy, nil
}

func (r *PersistentWorkerRepository) RemoveWorkerPoolOccupancy(poolId string) error {
	dbOps := func(tx *db.TxConnection) error {

		deleteOccupancyQ, err := db.NewQuery(tx, &model.WorkerPoolOccupancyEntity{}, r.Logger)
		if err != nil {
			return err
		}

		whereCond := map[string]interface{}{"WorkerPoolID": poolId}
		if err != nil {
			return err
		}

		deletionCnt, err := deleteOccupancyQ.Delete().Where(whereCond).Exec()
		if err != nil {
			r.Logger.Errorf("ReconRepo failed to delete occupancy entity with poolID '%s': %s", poolId, err)
			return err
		}

		r.Logger.Debugf("ReconRepo deleted '%d' occupancy entity with poolID '%s'", deletionCnt, poolId)
		return err
	}
	return db.Transaction(r.Conn, dbOps, r.Logger)
}
